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
	"regexp"
	"strconv"
	"time"

	"github.com/pelletier/go-toml"
	"go.mongodb.org/mongo-driver/bson"
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

//Data types
type reading struct {
	Ti string
	Te string
	Pr string
	Hu string
}
type outReading struct {
	Te string
	Pr string
	Hu string
}

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
}

func loginfo(info string) {
	logger.SetPrefix(time.Now().Format("2020-04-24 19:18:17") + " [INFO] ")
	logger.Print(info)
}

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

func dbHandler(w http.ResponseWriter, req *http.Request) {
	loginfo(fmt.Sprintf("Query on URL : %s with query %s", req.URL.Path, req.URL.RawQuery))
	// get db ellements older then 1 month
	// aggrégate
	// send back to db aggrégate
	// del old ellements
	json.NewEncoder(w).Encode(true)
}

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

// func pushData() error {

// 	return nil
// }

// func delData() error {

// 	return nil
// }

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

func main() {
	loginfo(fmt.Sprintf("Stating server on : %s", bind))
	http.HandleFunc("/sensordata", dataHandler)
	http.HandleFunc("/optimize_db", dbHandler)
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./static/js/"))))
	http.HandleFunc("/", mainpageHandler)
	log.Fatal(http.ListenAndServe(bind, nil))
}
