package Server

import (
	"VectoriaDB/Utils"
	"VectoriaDB/Vdb"
	"VectoriaDB/Vector"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

// RouteProvider is the global variable that contains all the routes
var RouteProvider *Routes

// NewRoutes returns a new Routes struct
func NewRoutes(db *Vdb.Vdb) *Routes {
	return &Routes{templates: template.Must(template.ParseGlob("templates/*.gohtml")), DB: db}
}

/* ROUTES */

// Index page
func (r *Routes) Index(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" && req.URL.Path == "/" {
		err := r.templates.ExecuteTemplate(w, "index.gohtml", NewData())
		if err != nil {
			panic(err.Error())
		}
	}
}

// Delete deletes a Collection
func (r *Routes) Delete(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost && strings.ToLower(req.URL.String()) == "/delete" {
		// Limit the size of the request
		req.Body = http.MaxBytesReader(w, req.Body, 5000)
		// Parse the form
		err := req.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error parsing form"))
			return
		}
		// load the request into the DeleteCollection via json decode
		dc := &DeleteCollection{}
		err = json.NewDecoder(req.Body).Decode(dc)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error decoding json"))
			return
		}
		// Call the function in the Vdb
		err = r.DB.DeleteCollection(dc.Name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		// Send the success or error message to the client
		w.WriteHeader(http.StatusOK)
		status := struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}{
			Status:  "success",
			Message: "Collection deleted",
		}
		json.NewEncoder(w).Encode(status)
		return
	}
}

// We have some static js / css files without showing the filelist
func static(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CreateCollection creates a new Collection
func (r *Routes) CreateCollection(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost && strings.ToLower(req.URL.String()) == "/createcollection" {
		// Limit the size of the request
		req.Body = http.MaxBytesReader(w, req.Body, 5000)
		// Parse the form
		err := req.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error parsing form"))
			return
		}
		// load the request into the CollectionCreator via json decode
		cc := &CollectionCreator{}
		err = json.NewDecoder(req.Body).Decode(cc)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error decoding json"))
			return
		}
		// Check if name is empty
		if cc.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Name is required"))
			return
		}
		// Check if Collection exists
		if _, ok := r.DB.Collections[cc.Name]; ok {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Collection with name " + cc.Name + " allready exists"))
			return
		}
		if cc.Wait {
			// Choose distance function from Distancefunction string
			if strings.ToLower(cc.DistanceFunction) != "euclid" {
				cc.DistanceFunction = "cosine"
			}
			err = r.DB.AddCollection(cc.Name, cc.Dimensions, cc.DistanceFunction)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			// Send the success or error message to the client
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Collection created"))
			return
		} else {
			// Create the Collection
			go r.DB.AddCollection(cc.Name, cc.Dimensions, cc.DistanceFunction)
			// Send the success or error message to the client
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Collection created"))
			return
		}

	}
	// Notice the user that the route is not found under given information
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
	return
}

// ListCollections lists all the Collections
func (r *Routes) ListCollections(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet && strings.ToLower(req.URL.String()) == "/listcollections" {
		// Create CollectionList type
		cl := &CollectionList{}
		// Get the Collections
		collections := r.DB.ListCollections()
		cl.Collections = collections
		// Send the collections to the client
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(cl)
		return
	}
	// Notice the user that the route is not found under given information
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
	return
}

// AddPoint adds a point to a Collection
func (r *Routes) AddPoint(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPut && strings.ToLower(req.URL.String()) == "/addpoint" {
		// Limit the size of the request
		req.Body = http.MaxBytesReader(w, req.Body, 1000000)
		// Parse the form
		err := req.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error parsing form"))
			return
		}
		// load the request into the Point via json decode
		p := &Point{}
		err = json.NewDecoder(req.Body).Decode(p)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error decoding json"))
			return
		}
		// Name, Vector are required
		if p.CollectionName == "" || p.Vector == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing required fields"))
			return
		}
		// Check if Collection exists
		if _, ok := r.DB.Collections[p.CollectionName]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Collection does not exist"))
			return
		}
		// Add the point to the Collection
		v := Vector.NewVector(p.Id, p.Vector, &p.Payload, p.CollectionName)
		err = r.DB.Collections[p.CollectionName].Insert(v)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		// Send the success or error message to the client
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Point added"))
		return
	}
	// Notice the user that the route is not found under given information
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
	return
}

// AddPointBatch adds a batch of points to a Collection
func (r *Routes) AddPointBatch(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPut && strings.ToLower(req.URL.String()) == "/addpointbatch" {
		// Limit the size of the request
		req.Body = http.MaxBytesReader(w, req.Body, 1000000)
		// Parse the form
		err := req.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error parsing form"))
			return
		}
		// load the request into the PointBatch via json decode
		pb := PointBatch{}
		err = json.NewDecoder(req.Body).Decode(&pb)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error decoding json"))
			return
		}
		// Name, Vector are required
		if pb.CollectionName == "" || pb.Points == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing required fields"))
			return
		}
		// Check if Collection exists
		if _, ok := r.DB.Collections[pb.CollectionName]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Collection does not exist"))
			return
		}
		// Add the points to the Collection
		for _, p := range pb.Points {
			d := p.Payload // This is no longer necessary from GO >= 1.22
			v := Vector.NewVector(p.Id, p.Vector, &d, pb.CollectionName)
			err = r.DB.Collections[pb.CollectionName].Insert(v)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		}
		// Send the success or error message to the client
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Points added"))
		return
	}
	// Notice the user that the route is not found under given information
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
	return
}

// DeletePoint deletes a point from a Collection
func (r *Routes) DeletePoint(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodDelete && strings.ToLower(req.URL.String()) == "/deletepoint" {
		// Limit the size of the request
		req.Body = http.MaxBytesReader(w, req.Body, 5000)
		// Parse the form
		err := req.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error parsing form"))
			return
		}
		// load the request into the DeletePoint via json decode
		dp := &DeletePoint{}
		err = json.NewDecoder(req.Body).Decode(dp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error decoding json"))
			return
		}
		// Check if Collection exists
		if _, ok := r.DB.Collections[dp.CollectionName]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Collection does not exist"))
			return
		}
		// Delete the point from the Collection
		err = r.DB.Collections[dp.CollectionName].Delete(dp.Id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		// Send the success or error message to the client
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Point deleted"))
		return
	}
	// Notice the user that the route is not found under given information
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
	return
}

// Search searches for the nearest neighbours of the given target vector
func (r *Routes) Search(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		// Parse the form
		err := req.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error parsing form"))
			return
		}
		// load the request into the Point via json decode
		p := &Point{}
		err = json.NewDecoder(req.Body).Decode(p)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Println(err.Error())
			w.Write([]byte("Error decoding json"))
			return
		}
		// Name, Vector are required
		if p.CollectionName == "" || p.Vector == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing required fields"))
			return
		}
		// Check if Collection exists
		if _, ok := r.DB.Collections[p.CollectionName]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Collection does not exist"))
			return
		}
		// Search for the nearest neighbours
		var queue *Utils.PriorityQueue
		if p.Depth == 0 {
			queue = Utils.NewPriorityQueue(3)
		} else {
			queue = Utils.NewPriorityQueue(p.Depth)
		}
		// Search for the nearest neighbours
		results := r.DB.Search(p.CollectionName, Vector.NewVector(p.Id, p.Vector, &p.Payload, ""), queue, p.MaxDistancePercent)
		// Create the SearchResult
		sr := &SearchResult{}
		for _, r := range results {
			sr.Results = append(sr.Results, &Result{Vector: r.Node.Vector, Distance: r.Distance})
		}
		// Send the results to the client
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(sr)
		return
	}
	// Notice the user that the route is not found under given information
	fmt.Println(req.Method, req.URL.String())
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
	return
}
