package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	Router *chi.Mux
}

func NewServer() *Server {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	return &Server{
		Router: router,
	}
}

func (s *Server) Serve() {
	// Register all Handler and Routes that we support
	Register(s)
	http.ListenAndServe(":8080", s.Router)
}

func Register(s *Server) {
	// Create bucket route
	// Refers to: ms3 create bucket {bucket-name}
	s.Router.Put("/bucket/{bucket-name}", CreateBucketHandler)

	// Delete bucket route
	// Refers to: ms3 delete bucket {bucket-name} or ms3 remove {bucket-name}
	s.Router.Delete("/bucket/{bucket-name}", DeleteBucketHandler)

	// List all available buckets route
	// Refers to: ms3 get buckets or ms3 list buckets
	s.Router.Get("/buckets", ListBucketsHandler)

	// Create object in default bucket
	// Refers to: ms3 create object {object-name}
	s.Router.Put("/default-bucket/{object-name}", CreateObjectHandler)

	// Create object in specific bucket
	// Refers to: ms3 create object bucket-name/object
	s.Router.Put("/{bucket-name}/{object-name}", CreateObjectHandler)

	// Delete object from default bucket
	// ms3 delete object {object-name} or ms3 delete object default-bucket/object // remove
	s.Router.Delete("/default-bucket/{object-name}", DeleteObjectHandler)

	// Delete object from named bucket
	// Refers to: ms3 delete object bucket-name/object
	s.Router.Delete("/{bucket-name}/{object-name}", DeleteObjectHandler)

	// Get all stored objects
	// Refers to: ms3 get objects or ms3 list objects
	s.Router.Get("/objects", ListAllObjectsHandler)

	// Get all objects and buckets
	// Refers to: ms3 list
	s.Router.Get("/all", ListAllHandler)
}

func CreateBucketHandler(w http.ResponseWriter, r *http.Request) {
	bName := r.PathValue("bucket-name")
	formatted := fmt.Sprintf("Create bucket %s | Similar to ms3 create bucket %s\n", bName, bName)
	w.Write([]byte(formatted))
}

func DeleteBucketHandler(w http.ResponseWriter, r *http.Request) {
	bName := r.PathValue("bucket-name")
	formatted := fmt.Sprintf("Delete bucket %s | Similar to ms3 delete bucket %s\n", bName, bName)
	w.Write([]byte(formatted))
}

func ListBucketsHandler(w http.ResponseWriter, r *http.Request) {
	buckets := []string{"old-bucket", "new-bucket", "bucket-1", "bucket-2", "bucket-3", "bucket-n"}
	formatted := fmt.Sprintf("List all available buckets | Similar to ms3 get buckets or ms3 list buckets\n%v\n", buckets)
	w.Write([]byte(formatted))
}

func CreateObjectHandler(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket-name")
	if bucketName == "" {
		CreateObjectDefaultHandler(w, r)
		return
	}
	objectName := r.PathValue("object-name")
	formatted := fmt.Sprintf("Create object %s | Similar to ms3 create object %s/%s or `ms3 upload object`\n", objectName, bucketName, objectName)
	w.Write([]byte(formatted))
}

func CreateObjectDefaultHandler(w http.ResponseWriter, r *http.Request) {
	objectName := r.PathValue("object-name")
	formatted := fmt.Sprintf("Create object %s | Similar to ms3 create object default-bucket/%s or `ms3 upload object`\n", objectName, objectName)
	w.Write([]byte(formatted))
}

func DeleteObjectHandler(w http.ResponseWriter, r *http.Request) {
	bucketName := r.PathValue("bucket-name")
	if bucketName == "" {
		DeleteObjectDefaultHandler(w, r)
		return
	}
	objectName := r.PathValue("object-name")
	formatted := fmt.Sprintf("Delete object %s | Similar to ms3 delete object %s/%s\n", objectName, bucketName, objectName)
	w.Write([]byte(formatted))
}

func DeleteObjectDefaultHandler(w http.ResponseWriter, r *http.Request) {
	objectName := r.PathValue("object-name")
	formatted := fmt.Sprintf("Delete object %s | Similar to ms3 delete object default-bucket/%s\n", objectName, objectName)
	w.Write([]byte(formatted))
}

func ListAllObjectsHandler(w http.ResponseWriter, r *http.Request) {
	objects := []string{"old-object", "new-object", "object-1", "object-2", "object-3", "object-n"}
	formatted := fmt.Sprintf("List all available objects | Similar to ms3 get objects or ms3 list objects\n%v\n", objects)
	w.Write([]byte(formatted))
}

func ListAllHandler(w http.ResponseWriter, r *http.Request) {
	objects := []string{"old-object", "new-object", "object-1", "object-2", "object-3", "object-n"}
	buckets := []string{"old-bucket", "new-bucket", "bucket-1", "bucket-2", "bucket-3", "bucket-n"}
	formatted := fmt.Sprintf("List all available components | Similar to ms3 list\n%v\n%v\n", buckets, objects)
	w.Write([]byte(formatted))
}
