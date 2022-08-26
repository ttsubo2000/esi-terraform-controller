package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	"k8s.io/klog/v2"
)

var Configurations []types.Configuration

func homePage(w http.ResponseWriter, r *http.Request) {
	klog.Info(w, "Welcome to the HomePage!")
	klog.Info("Endpoint Hit: homePage")
}

// HandleRequests is for creating a new instance of a mux router
func HandleRequests(clientState cacheObj.Store) {
	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/", homePage)
	myRouter.HandleFunc("/secrets", returnAllSecrets)
	myRouter.HandleFunc("/secret/{name}", returnSingleSecret)
	myRouter.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) {
		createNewSecret(w, r, clientState)
	}).Methods("POST")
	myRouter.HandleFunc("/secret/{name}", func(w http.ResponseWriter, r *http.Request) {
		deleteSecret(w, r, clientState)
	}).Methods("DELETE")

	myRouter.HandleFunc("/providers", returnAllProviders)
	myRouter.HandleFunc("/provider/{name}", returnSingleProvider)
	myRouter.HandleFunc("/provider", func(w http.ResponseWriter, r *http.Request) {
		createNewProvider(w, r, clientState)
	}).Methods("POST")
	myRouter.HandleFunc("/provider/{name}", func(w http.ResponseWriter, r *http.Request) {
		deleteProvider(w, r, clientState)
	}).Methods("DELETE")

	myRouter.HandleFunc("/configurations", returnAllConfigurations)
	myRouter.HandleFunc("/configuration/{name}", returnSingleConfiguration)
	myRouter.HandleFunc("/configuration", func(w http.ResponseWriter, r *http.Request) {
		createNewConfiguration(w, r, clientState)
	}).Methods("POST")
	myRouter.HandleFunc("/configuration/{name}", func(w http.ResponseWriter, r *http.Request) {
		deleteConfiguration(w, r, clientState)
	}).Methods("DELETE")
	klog.Fatal(http.ListenAndServe(":10000", myRouter))
}
