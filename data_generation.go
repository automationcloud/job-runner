package jobrunner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func GenerateData(jibUrl string, config JibConfig, client *http.Client) (data map[string]interface{}, err error) {
	// fmt.Println("config is", config, "this is it")
	jibJson, err := json.Marshal(config)
	if err != nil {
		return
	}

	req, err := http.NewRequest(
		"POST",
		jibUrl,
		bytes.NewBuffer(jibJson),
	)
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return
	}

	data = make(map[string]interface{})
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return
	}

	if err != nil {
		return
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	fmt.Println(keys)
	return
}
