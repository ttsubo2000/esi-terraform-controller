package rest

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	"k8s.io/klog/v2"
)

var Secrets []types.Secret

func returnAllSecrets(w http.ResponseWriter, r *http.Request) {
	klog.Info("Endpoint Hit: returnAllSecrets")
	json.NewEncoder(w).Encode(Secrets)
}

func returnSingleSecret(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["name"]

	for _, secret := range Secrets {
		if secret.ObjectMeta.Name == key {
			json.NewEncoder(w).Encode(secret)
		}
	}
}

func createNewSecret(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var secret types.Secret
	json.Unmarshal(reqBody, &secret)
	clientState.Add(&secret)
	Secrets = append(Secrets, secret)

	json.NewEncoder(w).Encode(secret)
}

func deleteSecret(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var secret types.Secret
	json.Unmarshal(reqBody, &secret)
	clientState.Delete(&secret)

	vars := mux.Vars(r)
	name := vars["name"]

	for index, secret := range Secrets {
		if secret.ObjectMeta.Name == name {
			Secrets = append(Secrets[:index], Secrets[index+1:]...)
		}
	}
}
