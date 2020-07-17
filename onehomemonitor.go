package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/pelletier/go-toml"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Configuration utils
var bind string
var staticPath string
var logFile string

type mongoDBConConf struct {
	Username    string
	Password    string
	ClusterFQDN string
}
type configuration struct {
	MongoDBConnexionConfiguration mongoDBConConf
}

var runningConf configuration
var confFile string
var logger *log.Logger
var dbOptimizingState string
var dbOptimizingErr error

//Data types
type reading struct {
	Ti string `bson:"ti"`
	Te string `bson:"te"`
	Pr string `bson:"pr"`
	Hu string `bson:"hu"`
}
type outReading struct {
	Te string
	Pr string
	Hu string
}

/*
Initialization function deals with argument and configuration file parsing.
*/
func init() {
	// Read running args
	flag.StringVar(&confFile, "conf_file", "./onehomemonitor.toml", "Path to configuration file")
	flag.StringVar(&bind, "bind", ":8080", "IP:Port to bind listen socket")
	flag.StringVar(&staticPath, "static_path", "./static", "Path to folder holding static files")
	flag.StringVar(&logFile, "log_file", "os.stdout", "Path to logging file")
	flag.Parse()

	// Prépare logging file
	if logFile == "os.stdout" {
		logger = log.New(os.Stdout, "", log.Lshortfile)
	} else {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		logger = log.New(f, "", log.Lshortfile)
	}

	// read configuration file
	confFileData, err := ioutil.ReadFile(confFile)
	if err != nil {
		log.Fatal(err)
	}
	toml.Unmarshal(confFileData, &runningConf)
	dbOptimizingState = "stoped"
}

/*
Logging functions, to deal with information and error messages.
*/
func loginfo(info string) {
	logger.SetPrefix(time.Now().Format("2020-04-24 19:18:17") + " [INFO] ")
	logger.Print(info)
}
func logerror(info string) {
	logger.SetPrefix(time.Now().Format("2020-04-24 19:18:17") + " [ERR] ")
	logger.Print(info)
}

/*
Utils functions:
  - utilDataStringAvr: takes an array of strings, converts it to ints and sends back the average of the values.
    result is sent back in string form.
*/
func utilDataStringAvr(t []string) string {
	var total float64 = 0
	for _, v := range t {
		vf, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return "0"
		}
		total += vf
	}
	return fmt.Sprintf("%.2f", (total / float64(len(t))))
}

/*
 This function will to all the optimization processe for the database.
 It the only function that can change the global dbOptimizingState variable.
  1 - All the data older then a month ago is retreaved.
  2 - Data is aggregated to the 3 hours points.
  3 - The aggregated date (just calculated) is added to the database (if it's not already there)
  4 - All the retreaved points are deleted.
 Thus only the aggregated data remains in the data base.
*/
func dbOptimize() {
	startOptimization := time.Now()
	// get db ellements older then 1 month
	dbOptimizingState = "fetching_data"
	monthAgo := time.Now().AddDate(0, -1, 0).Unix()
	queryFilter := bson.M{
		"ti": bson.M{
			"$lte": strconv.Itoa(int(monthAgo)),
		},
	}
	data, err := getData(queryFilter)
	if err != nil {
		dbOptimizingState = "failed"
		dbOptimizingErr = err
		logerror(fmt.Sprintf("Error occured while fetching data : %s", err))
	}
	dbOptimizingState = "cleaning_data"
	// Go throught all the data all the sensors and aggregate the data.
	for senName, senData := range data {
		loginfo(fmt.Sprintf("Cleaning data for %s", senName))
		aggData := map[int64][]reading{}
		addData := []reading{}
		for _, r := range senData {
			i, err := strconv.ParseInt(r.Ti, 10, 64)
			if err != nil {
				dbOptimizingState = "failed"
				dbOptimizingErr = err
				logerror(fmt.Sprintf("Error occured while converting data : %s", err))
			}
			aggDate := time.Unix(i, 0).Truncate(time.Hour * 3).Unix()
			if _, pres := aggData[aggDate]; pres {
				aggData[aggDate] = append(aggData[aggDate], r)
			} else {
				aggData[aggDate] = []reading{r}
			}
		}
		for date, readings := range aggData {
			tes := []string{}
			prs := []string{}
			hus := []string{}
			for _, r := range readings {
				tes = append(tes, r.Te)
				prs = append(prs, r.Pr)
				hus = append(hus, r.Hu)
			}
			aggReading := reading{
				Ti: strconv.FormatInt(date, 10),
				Te: utilDataStringAvr(tes),
				Pr: utilDataStringAvr(prs),
				Hu: utilDataStringAvr(hus),
			}
			needsToBeSent := true
			for s, oriRead := range senData {
				if reflect.DeepEqual(oriRead, aggReading) {
					// the aggegated data is already in the recieved data
					senData = append(senData[:s], senData[s+1:]...)
					needsToBeSent = false
					break
				}
			}
			if needsToBeSent {
				addData = append(addData, aggReading)
			}
		}
		// Send aggReadings selected to addData
		loginfo(fmt.Sprintf("Sending %d new data for %s", len(addData), senName))
		err = dbInteractMany(pushManyData, senName, addData)
		if err != nil {
			dbOptimizingState = "failed"
			dbOptimizingErr = err
			logerror(fmt.Sprintf("Error occured while adding aggregated data : %s", err))
		}
		//Delete reamaning data
		loginfo(fmt.Sprintf("Removing %d old data form %s", len(senData), senName))
		err = dbInteractMany(delManyData, senName, senData)
		if err != nil {
			dbOptimizingState = "failed"
			dbOptimizingErr = err
			logerror(fmt.Sprintf("Error occured while deleting data : %s", err))
		}
	}
	durationOptimization := time.Now().Sub(startOptimization)
	dbOptimizingState = "stoped"
	loginfo(fmt.Sprintf("Optimized the database in %s", durationOptimization))
}

/*
  Functions to intercate with the database. Functions includes connections, data insertion/deletation and fetching.
  Database name is hardcoded.
*/
func connectToDB() (*mongo.Client, context.Context, context.CancelFunc, error) {
	mongoConURL := fmt.Sprintf("mongodb+srv://%s:%s@%s/test?retryWrites=true&w=majority",
		runningConf.MongoDBConnexionConfiguration.Username,
		runningConf.MongoDBConnexionConfiguration.Password,
		runningConf.MongoDBConnexionConfiguration.ClusterFQDN)
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoConURL))
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		return nil, nil, cancel, err
	}
	return client, ctx, cancel, nil
}

func dbIntercat(colName string) (*mongo.Collection, context.Context, context.CancelFunc, error) {
	client, ctx, cancel, err := connectToDB()
	if err != nil {
		return nil, nil, nil, err
	}
	db := client.Database("onehomesensor").Collection(colName) //database name hardcoded, as in collecting project
	return db, ctx, cancel, nil
}

func getData(queryFilter bson.M) (map[string][]reading, error) {
	var out = map[string][]reading{}
	client, ctx, cancel, err := connectToDB()
	defer cancel()
	if err != nil {
		return nil, err
	}
	// Register sensors readings
	db := client.Database("onehomesensor") //database name hardcoded, as in collecting project
	colNames, err := db.ListCollectionNames(context.Background(), bson.D{})
	if err != nil {
		return nil, err
	}
	for _, v := range colNames {
		out[v] = []reading{}
		coll := db.Collection(v)
		cur, err := coll.Find(ctx, queryFilter)
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var r reading
			err := cur.Decode(&r)
			if err != nil {
				return nil, err
			}
			out[v] = append(out[v], r)
		}
	}
	_ = client.Disconnect(ctx) // Discard error
	return out, nil
}

// In order to simplify the code I can pass the function in the parameters of an wrapper function
type interactDBfunc func(string, []reading) error

func dbInteractMany(fn interactDBfunc, senName string, data []reading) error {
	err := fn(senName, data)
	if err != err {
		return err
	}
	return nil
}

func pushManyData(colName string, r []reading) error {
	db, ctx, cancel, err := dbIntercat(colName)
	defer cancel()
	if err != nil {
		return err
	}
	interfaceData := make([]interface{}, len(r))
	for i, v := range r {
		interfaceData[i] = v
	}
	_, err = db.InsertMany(ctx, interfaceData)
	return err
}

func delManyData(colName string, r []reading) error {
	db, ctx, cancel, err := dbIntercat(colName)
	defer cancel()
	if err != nil {
		return err
	}
	ti := make([]string, len(r))
	for i, s := range r {
		ti[i] = s.Ti
	}
	delFilter := bson.D{primitive.E{
		Key:   "ti",
		Value: bson.D{primitive.E{Key: "$in", Value: ti}},
	}}
	_, err = db.DeleteMany(ctx, delFilter)
	return err
}

/*
   Web server : Handles requests form clients.
	 - mainpageHandler : will send back favicon or home page
	 - dataHandler : sends back data form the database.
	 - dbHandler : handle demends to optimize the database.
*/

func mainpageHandler(w http.ResponseWriter, req *http.Request) {
	loginfo(fmt.Sprintf("Query on URL : %s", req.URL.Path))
	var faviconTarget = regexp.MustCompile("/favicon.*")
	var isFaviconTarget = faviconTarget.MatchString(req.URL.Path)
	if isFaviconTarget {
		http.ServeFile(w, req, filepath.Join(staticPath, "favicon.ico"))
	} else {
		http.ServeFile(w, req, filepath.Join(staticPath, "home.html"))
	}
}

func dataHandler(w http.ResponseWriter, req *http.Request) {
	loginfo(fmt.Sprintf("Query on URL : %s with query %s", req.URL.Path, req.URL.RawQuery))

	// Prepare filter arguments
	// f : from, t : to and l : limite
	urlQuery := req.URL.Query()
	var queryFilter bson.M
	if urlQuery.Get("f") != "" && urlQuery.Get("t") != "" {
		queryFilter = bson.M{
			"ti": bson.M{
				"$gte": urlQuery.Get("f"),
				"$lte": urlQuery.Get("t"),
			},
		}
	} else if urlQuery.Get("f") != "" {
		queryFilter = bson.M{
			"ti": bson.M{
				"$gte": urlQuery.Get("f"),
			},
		}
	} else {
		queryFilter = bson.M{}
	}
	var res = map[string]map[int]outReading{}
	data, err := getData(queryFilter)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	for c, v := range data {
		res[c] = map[int]outReading{}
		for _, r := range v {
			time, err := strconv.Atoi(r.Ti)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			res[c][time] = outReading{r.Te, r.Pr, r.Hu}
		}
	}
	json.NewEncoder(w).Encode(res)
}

// This Handler will read the state of the database optimisation task and send it back to the client.
// If the data based optimisation task is not started, this process will start one.
func dbHandler(w http.ResponseWriter, req *http.Request) {
	loginfo(fmt.Sprintf("Query on URL : %s with query %s", req.URL.Path, req.URL.RawQuery))
	// Initialization of the out state returned
	outState := map[string]string{
		"state": "unknown",
		"msg":   "State of the database optimisation processe unknown",
	}
	if dbOptimizingState == "stoped" || dbOptimizingState == "starting" {
		dbOptimizingState = "starting"
		outState["state"] = "starting"
		outState["msg"] = "A Database optimizing processe just started."
		go dbOptimize()
	} else if dbOptimizingState == "fetching_data" {
		outState["state"] = "Fetaching data from data base."
		outState["msg"] = "All the data from the database is been fetch to be analysed."
	} else if dbOptimizingState == "cleaning_data" {
		outState["state"] = "Cleaning the data from the database"
		outState["msg"] = "Calculation to optimise the database are been made."
	} else if dbOptimizingState == "failed" {
		dbOptimizingState = "stoped"
		http.Error(w, dbOptimizingErr.Error(), 500)
	}
	json.NewEncoder(w).Encode(outState)
}

func main() {
	loginfo(fmt.Sprintf("Stating server on : %s", bind))
	http.HandleFunc("/sensordata", dataHandler)
	http.HandleFunc("/optimize_db", dbHandler)
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./static/js/"))))
	http.HandleFunc("/", mainpageHandler)
	log.Fatal(http.ListenAndServe(bind, nil))
}
