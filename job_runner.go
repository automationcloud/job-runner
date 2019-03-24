package jobrunner

import (
	"fmt"
	"net/http"
	"time"

	//witness "github.com/1602/witness"

	cl "github.com/automationcloud/client-go"
)

var jobRunner *JobRunner

func NewRunner(httpClient *http.Client, apiKey, baseUrl string) JobRunner {
	return JobRunner{
		httpClient: httpClient,
		cl.ApiClient{
			Client:    httpClient,
			BaseUrl:   baseUrl,
			SecretKey: apiKey,
		},
	}
}

type JobRunner struct {
	apiClient  *cl.ApiClient
	httpClient *http.Client
	DomainId   string
	InputData  map[string]interface{}
}

type JibConfig = map[string]interface{}

type JobRun struct {
	ServiceId   string    `json:"serviceId"`
	DomainId    string    `json:"domainId"`
	JibConfig   JibConfig `json:"jibConfig"`
	CallbackUrl string    `json:"callbackUrl"`
	HowMany     int       `json:"howMany"`
}

func (jr *JobRunner) RunJob(jobRun JobRun) (job cl.Job, err error) {
	inputData, err := GenerateData(jobRun.JibConfig, jr.httpClient)
	if err != nil {
		return job, err
	}
	jr.InputData = inputData
	jr.DomainId = jobRun.DomainId

	prot, err := jr.apiClient.GetProtocol()
	if err != nil {
		return job, err
	}

	createWithInputs := FilterInputs(prot.Domains[jobRun.DomainId], inputData)

	for i := 0; i < jobRun.HowMany; i++ {
		job, err := jr.apiClient.CreateJob(jobRun.ServiceId, createWithInputs, MakeCallbackUrl(jobRun.CallbackUrl, jobRun.DomainId))
		if err != nil {
			return job, err
		}
	}

	return job, err
}

func MakeCallbackUrl(url, domainId string) string {
	if url == "" {
		return ""
	}

	return url + "?domainId=" + domainId
}

func FilterInputs(d cl.Domain, i map[string]interface{}) (result map[string]interface{}) {
	result = make(map[string]interface{})
	for key, _ := range d.Inputs {
		data, present := i[key]
		if present {
			result[key] = data
		}
	}
	return
}

func Process(job *cl.Job, pause int) bool {
	switch job.State {
	case "processing":
		return true
	case "awaitingInput":
		jobRunner.CreateInput(job, pause)
		return true
	case "awaitingTds":
		return false
	default:
		return false
	}
}

func GetFromOutput(job *cl.Job, key string, method string) (data interface{}, err error) {
	output, err := job.GetOutput(key)
	if err != nil {
		return
	}

	switch method {
	case "Consent":
		return output.Data, nil
	case "SelectOne":
		arr := output.Data.([]interface{})
		return arr[0], nil
	}

	return
}

func (jr *JobRunner) CreateInput(job *cl.Job, pause int) {
	var data interface{}
	var err error
	var ok bool
	var prot cl.Protocol

	if jr.InputData != nil {
		data, ok = jr.InputData[job.AwaitingInputKey]
		if !ok {
			prot, err = jr.apiClient.GetProtocol()
			if err != nil {
				job.Cancel()
				panic(err)
			}
			inputDef, found := prot.Domains[jr.DomainId].Inputs[job.AwaitingInputKey]
			if !found {
				job.Cancel()
				panic("unexpected awaitingInputKey " + job.AwaitingInputKey)
			}
			data, err = GetFromOutput(job, inputDef.SourceOutputKey, inputDef.InputMethod)
			if err != nil {
				job.Cancel()
				panic(err)
			}
		}
	}

	fmt.Printf("Waiting %d seconds before submitting an input for key %s", pause, job.AwaitingInputKey)
	time.Sleep(time.Duration(pause) * time.Second)

	input, err := job.CreateInput(cl.InputCreationRequest{Key: job.AwaitingInputKey, Data: data})
	fmt.Println(input)
	return
}
