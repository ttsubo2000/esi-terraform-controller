package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"k8s.io/klog/v2"
)

func homePage(w http.ResponseWriter, r *http.Request) {
	klog.Info(w, "Welcome to the HomePage!")
	klog.Info("Endpoint Hit: homePage")
}

// HandleRequests is for creating a new instance of a mux router
func HandleRequests(clientState cacheObj.Store) {
	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/", homePage)

	myRouter.HandleFunc("/secrets", func(w http.ResponseWriter, r *http.Request) {
		returnAllSecrets(w, r, clientState)
	}).Methods("GET")
	myRouter.HandleFunc("/secret/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		returnSingleSecret(w, r, clientState)
	}).Methods("GET")
	myRouter.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) {
		createNewSecret(w, r, clientState)
	}).Methods("POST")
	myRouter.HandleFunc("/secret/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		updateSecret(w, r, clientState)
	}).Methods("PUT")
	myRouter.HandleFunc("/secret/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		deleteSecret(w, r, clientState)
	}).Methods("DELETE")

	myRouter.HandleFunc("/providers", func(w http.ResponseWriter, r *http.Request) {
		returnAllProviders(w, r, clientState)
	}).Methods("GET")
	myRouter.HandleFunc("/provider/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		returnSingleProvider(w, r, clientState)
	}).Methods("GET")
	myRouter.HandleFunc("/provider", func(w http.ResponseWriter, r *http.Request) {
		createNewProvider(w, r, clientState)
	}).Methods("POST")
	myRouter.HandleFunc("/provider/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		updateProvider(w, r, clientState)
	}).Methods("PUT")
	myRouter.HandleFunc("/provider/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		deleteProvider(w, r, clientState)
	}).Methods("DELETE")

	myRouter.HandleFunc("/configurations", func(w http.ResponseWriter, r *http.Request) {
		returnAllConfigurations(w, r, clientState)
	}).Methods("GET")
	myRouter.HandleFunc("/configuration/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		returnSingleConfiguration(w, r, clientState)
	}).Methods("GET")
	myRouter.HandleFunc("/configuration", func(w http.ResponseWriter, r *http.Request) {
		createNewConfiguration(w, r, clientState)
	}).Methods("POST")
	myRouter.HandleFunc("/configuration/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		updateConfiguration(w, r, clientState)
	}).Methods("PUT")
	myRouter.HandleFunc("/configuration/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		deleteConfiguration(w, r, clientState)
	}).Methods("DELETE")

	klog.Fatal(http.ListenAndServe(":10000", myRouter))
}
