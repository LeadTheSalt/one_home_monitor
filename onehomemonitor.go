package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/pelletier/go-toml"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var bind string
var static_path string
var log_file string

type MongoDB_con_conf struct {
	Username    string
	Password    string
	ClusterFQDN string
}
type Configuration struct {
	MongoDBConnexionConfiguration MongoDB_con_conf
}

var running_conf Configuration
var conf_file string
var logger *log.Logger

func init() {
	// Read running args
	flag.StringVar(&conf_file, "conf_file", "./onehomemonitor.toml", "Path to configuration file")
	flag.StringVar(&bind, "bind", ":8080", "IP:Port to bind listen socket")
	flag.StringVar(&static_path, "static_path", "./static", "Path to folder holding static files")
	flag.StringVar(&log_file, "log_file", "os.stdout", "Path to logging file")
	flag.Parse()

	// Prépare logging file
	if log_file == "os.stdout" {
		logger = log.New(os.Stdout, "", log.Lshortfile)
	} else {
		f, err := os.OpenFile(log_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		logger = log.New(f, "", log.Lshortfile)
	}

	// read configuration file
	conf_file_data, err := ioutil.ReadFile(conf_file)
	if err != nil {
		log.Fatal(err)
	}
	toml.Unmarshal(conf_file_data, &running_conf)
}

func loginfo(info string) {
	logger.SetPrefix(time.Now().Format("2020-04-24 19:18:17") + " [INFO] ")
	logger.Print(info)
}

func mainpageHandler(w http.ResponseWriter, req *http.Request) {
	loginfo(fmt.Sprintf("Query on URL : %s", req.URL.Path))
	var favicon_target = regexp.MustCompile("/favicon.*")
	var is_favicon_target = favicon_target.MatchString(req.URL.Path)
	if is_favicon_target {
		http.ServeFile(w, req, filepath.Join(static_path, "favicon.ico"))
	} else {
		http.ServeFile(w, req, filepath.Join(static_path, "home.html"))
	}
}

func dataHandler(w http.ResponseWriter, req *http.Request) {
	loginfo(fmt.Sprintf("Query on URL : %s with query %s", req.URL.Path, req.URL.RawQuery))

	type Reading struct {
		Ti string
		Te string
		Pr string
		Hu string
	}
	type OutReading struct {
		Te string
		Pr string
		Hu string
	}

	// Prepare filter arguments
	// f : from, t : to and l : limite
	url_query := req.URL.Query()
	var query_filter bson.M
	if url_query.Get("f") != "" && url_query.Get("t") != "" {
		query_filter = bson.M{
			"ti": bson.M{
				"$gte": url_query.Get("f"),
				"$lte": url_query.Get("t"),
			},
		}
	} else if url_query.Get("f") != "" {
		query_filter = bson.M{
			"ti": bson.M{
				"$gte": url_query.Get("f"),
			},
		}
	} else {
		query_filter = bson.M{}
	}

	// Prepare db connection
	var res = map[string]map[int]OutReading{}
	mongo_con_url := fmt.Sprintf("mongodb+srv://%s:%s@%s/test?retryWrites=true&w=majority",
		running_conf.MongoDBConnexionConfiguration.Username,
		running_conf.MongoDBConnexionConfiguration.Password,
		running_conf.MongoDBConnexionConfiguration.ClusterFQDN)
	client, err := mongo.NewClient(options.Client().ApplyURI(mongo_con_url))
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}

	// Register sensors readings
	db := client.Database("onehomesensor") //database name hardcoded, as in collecting project
	col_names, err := db.ListCollectionNames(context.Background(), bson.D{})
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	for _, v := range col_names {
		res[v] = map[int]OutReading{}
		coll := db.Collection(v)
		cur, err := coll.Find(ctx, query_filter)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		defer cur.Close(ctx)
		//readings = make(map(int)interface{})
		for cur.Next(ctx) {
			var r Reading
			err := cur.Decode(&r)
			if err != nil {
				log.Fatal(err)
			}
			time, err := strconv.Atoi(r.Ti)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			res[v][time] = OutReading{r.Te, r.Pr, r.Hu}
		}
	}
	_ = client.Disconnect(ctx) // Discard error
	json.NewEncoder(w).Encode(res)
}

func main() {
	loginfo(fmt.Sprintf("Stating server on : %s", bind))
	http.HandleFunc("/sensordata", dataHandler)
	http.HandleFunc("/", mainpageHandler)
	log.Fatal(http.ListenAndServe(bind, nil))
}
