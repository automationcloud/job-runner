// Package jobrunner provides higher-level abstraction of automation cloud API
// which allows to use protocol in order to automate in-flow input requests
package jobrunner

import (
	"errors"
	"math"
	"net/http"

	cl "github.com/automationcloud/client-go"
)

// NewRunner create a new JobRunner.
func NewRunner(httpClient *http.Client, apiKey, baseUrl, jibUrl string) JobRunner {
	return JobRunner{
		httpClient: httpClient,
		JibUrl:     jibUrl,
		apiClient:  cl.NewApiClient(httpClient, apiKey).WithBaseURL(baseUrl),
	}
}

// JobRunner manages job.
type JobRunner struct {
	apiClient  *cl.ApiClient
	httpClient *http.Client
	DomainId   string
	Job        *cl.Job
	JibUrl     string `json:"jibUrl"`
	InputData  map[string]interface{}
}

// JobRun is an instruction required to run a job using JobRunner, options are:
// - ServiceId: id of automation service, required
// - DomainId: id of domain, required
// - JibConfig: job input bundler (jib) configuration, required
// - CallbackUrl: callback url for webhook
// - HowMany: how many jobs with the same input data to run (used to test concurrency), defaults to 1
type JobRun struct {
	ServiceId        string    `json:"serviceId"`
	DomainId         string    `json:"domainId"`
	JibConfig        JibConfig `json:"jibConfig"`
	CallbackUrl      string    `json:"callbackUrl,omitempty"`
	OversupplyInputs bool      `json:"oversupplyInputs"`
	HowMany          int       `json:"howMany"`
}

// RunJob create automation job which then will be stored in JobRunner object for further control.
func (jr *JobRunner) RunJob(jobRun JobRun) (job cl.Job, err error) {
	inputData, err := GenerateData(jr.JibUrl, jobRun.JibConfig, jr.httpClient)
	if err != nil {
		return job, err
	}
	jr.InputData = inputData
	jr.DomainId = jobRun.DomainId

	jcr := cl.JobCreationRequest{
		ServiceId:   jobRun.ServiceId,
		CallbackUrl: makeCallbackUrl(jobRun.CallbackUrl, jobRun.DomainId),
		Data:        inputData,
	}

	/*
		if jobRun.OversupplyInputs {
			prot, err := jr.apiClient.GetProtocol()
			if err != nil {
				return job, err
			}

			jcr.Data = filterInputs(prot.Domains[jobRun.DomainId], inputData)
		} else {
			jcr.Data = make(map[string]interface{})
		}
	*/

	for i := 0; i < int(math.Max(1.0, float64(jobRun.HowMany))); i++ {
		job, err = jr.apiClient.CreateJob(jcr)
		if err != nil {
			return job, err
		}
		// TODO: make working with multiple jobs
		jr.Job = &job
	}

	return job, err
}

// Resume job initializes jobrunner instance with running job.
func (jr *JobRunner) ResumeJob(jobId, domainId string) (err error) {
	job, err := jr.apiClient.FetchJob(jobId)
	jr.DomainId = domainId
	jr.Job = &job
	return
}

func makeCallbackUrl(url, domainId string) string {
	if url == "" {
		return ""
	}

	return url + "?domainId=" + domainId
}

/*
func filterInputs(d cl.Domain, i map[string]interface{}) (result map[string]interface{}) {
	result = make(map[string]interface{})
	for key, _ := range d.Inputs {
		data, present := i[key]
		if present {
			result[key] = data
		}
	}
	return
}
*/

func getFromOutput(job *cl.Job, key string, method string) (data interface{}, err error) {
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

// CreateInput makes an attempt to create input automatically.
// It uses job output and domain type definitions in order to do so.
// For example, it can send "finalPriceConsent" based on "finalPrice" output, if domain
// defines "finalPriceConsent" input with "finalPrice" as `sourceOutputKey` and "Consent" and `inputMethod`
func (jr *JobRunner) CreateInput() (err error) {
	var data interface{}
	var ok bool
	var prot *cl.Protocol

	if jr.Job == nil {
		return errors.New("job runner is not ready to create input: no job created or resumed")
	}

	if jr.InputData != nil {
		data, ok = jr.InputData[jr.Job.AwaitingInputKey]
	}

	if !ok {
		prot, err = jr.apiClient.GetProtocol()
		if err != nil {
			return err
		}
		inputDef, found := prot.Domains[jr.DomainId].Inputs[jr.Job.AwaitingInputKey]
		if !found {
			return errors.New("unexpected awaitingInputKey " + jr.Job.AwaitingInputKey)
		}
		data, err = getFromOutput(jr.Job, inputDef.SourceOutputKey, inputDef.InputMethod)
		if err != nil {
			return err
		}
	}

	_, err = jr.Job.CreateInput(cl.InputCreationRequest{Key: jr.Job.AwaitingInputKey, Data: data})
	return
}
