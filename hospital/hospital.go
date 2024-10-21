package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

)

type Share struct {
	Share int
}

type Patient struct {
	Port int
	PortsList []int
}


var client *http.Client
var patients []int
var data int
var port int
var totalPatients int
var registeredPatients int
var receivedShares int

func Patients(w http.ResponseWriter, req *http.Request) {
	log.Println(port, ": Hospital received /patients")
	if req.Method == "POST" {
		log.Println(port, ": Hospital received POST /patients")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(port, ": Error when reading request body during /patients:", err)
			w.WriteHeader(418)
			return
		}
		receivedPort := &Patient{}
		err = json.Unmarshal(body, receivedPort)
		if err != nil {
			log.Fatal(port, ": Error when unmarshalling port:", err)
			w.WriteHeader(418)
			return
		}
		log.Println(port, ": Registered new patient at port", receivedPort.Port)
		patients = append(patients, receivedPort.Port)
		registeredPatients++
		if registeredPatients == totalPatients {
			sendPorts()
		}
		w.WriteHeader(200)
	}
}

func Shares(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		log.Println(port, ": Hospital received POST /shares")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(port, ": Error when reading request body during /shares:", err)
			w.WriteHeader(418)
			return
		}
		share := &Share{}
		err = json.Unmarshal(body, share)
		if err != nil {
			log.Fatal(port, ": Error when unmarshalling share:", err)
			w.WriteHeader(418)
			return
		}
		data = data + share.Share
		receivedShares++
		log.Println(port, ": Hospital received share", share.Share, ", total of", receivedShares)

		if receivedShares == totalPatients {
			log.Println("Computation finished: The final value is", data)
		}
		w.WriteHeader(200)
	}
}

func StringifyPort(port int) string {
	return fmt.Sprintf(":%d", port)
}

func hospitalServer() {
	log.Println(port, ": Creating hospital server")

	mux := http.NewServeMux()
	mux.HandleFunc("/patient", Patients)
	mux.HandleFunc("/shares", Shares)

	err := http.ListenAndServeTLS(StringifyPort(port), "server.crt", "server.key", mux)
	if err != nil {
		log.Fatal(err)
	}
}

func sendPorts() {
	log.Println(port, ": Sending ports to patients")
	for i, p := range patients {
		otherPatients := make([]int, len(patients))
		copy(otherPatients, patients)
		otherPatients[i] = otherPatients[len(otherPatients)-1]
		otherPatients = otherPatients[:len(otherPatients)-1]

		log.Println(port, ": Sending ports", otherPatients, " to", p)
		url := fmt.Sprintf("https://localhost:%d/patients", p)
		patientPorts := Patient{
			PortsList: otherPatients,
		}

		b, err := json.Marshal(patientPorts)
		if err != nil {
			log.Fatal(port, ": Error when marshalling patientPorts:", err)
		}

		response, err := client.Post(url, "application/json", bytes.NewReader(b))
		if err != nil {
			log.Fatal(port, ": Error when posting patientPorts to", p, ":", err)
		}
		log.Println(port, ": Sent ports to ", p, ". Received response code", response.Status)

	}
}

func main() {

	flag.IntVar(&port, "port", 8080, "port of hospital")
	flag.IntVar(&totalPatients, "t", 3, "the total amount of patients")

	flag.Parse()

	//     rand.Seed(time.Now().UnixNano())
	data = 0

	cert, err := ioutil.ReadFile("server.crt")
	if err != nil {
		log.Fatal(err)
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)

	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}

	go hospitalServer()

	for {
	}

}
