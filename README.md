[![Build Status](https://travis-ci.org/LeadTheSalt/one_home_monitor.svg?branch=master)](https://travis-ci.org/LeadTheSalt/one_home_monitor)

# One Home Monitor 
This peace of sofware aims to read data stored by onehomesensor projet (on my github page). It is web server writen thanks in GO, it provides a web page with graphs and a backend to get information in MongoDB storage. 

## Usage 
```
onehomeminitor [OPTIONS]
```
Navigate to http://localhost:8080/

Options are :
  * conf_file : path to configuration file, mandatory for mongoDB connection
  * bind : IP and port to bind on, default is ":8080"
  * static_path : path to accès the static elements (home page and favicon)
  * log_file : custom logging file, default is standart stdout.

Backend part is accessible on url  http://localhost:8080/sensordata?f=[START_TIMESTAMP]&t=[END_TIMESTAMP]. It will send all reading between start and end timestamps. 

## Integration / Instalation
I advice to use Docker. Docker file is provided. TODO
```
docker build ...
docker run ... 
```

Otherwise:
```
git clone 
cd onehomemonitor
go build onemonitor 
./onemonitor
```

## Configuration file 
```
[MongoDBAtlasConnection]
username = # username set for MongoDB
password = # password set for MongoDB
clusterfqdn = # fqdn to MongoDB server 

```

## Extra Documenation/Help
https://www.alexedwards.net/blog/serving-static-sites-with-go  
https://stackoverflow.com/questions/26152993/go-logger-to-print-timestamp  
http://decouvric.cluster013.ovh.net/golang/gestion-des-erreurs-en-go.html  
https://stackoverflow.com/questions/55352362/filter-in-golang-mongodb   
https://kb.objectrocket.com/mongo-db/how-to-construct-mongodb-queries-from-a-string-using-golang-551  
https://github.com/chartjs/Chart.js  
https://getuikit.com/docs/introduction  