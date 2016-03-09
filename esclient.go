package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type EsResponseHit struct {
	Id         string                 `json:"_id"`
	Score      float64                `json:"_score"`
	Source     map[string]interface{} `json:"_source"`
	Hightlight map[string][]string    `json:"highlight"`
}

type EsResponseHits struct {
	Hits     []EsResponseHit `json:"hits"`
	Total    int             `json:"total"`
	MaxScore float64         `json:"max_score"`
}

type EsResponse struct {
	Hits     EsResponseHits `json:"hits"`
	Took     int            `json:"took"`
	TimedOut bool           `json:"timed_out"`
}

type EsClient struct {
	EsUrl          string
	ConnectTimeout time.Duration
}

type EsQueryOptions struct {
	Query      string
	NumResults int
	StartTime  time.Time
	EndTime    time.Time
	Show       bool
}

func (c *EsClient) Search(queryOpts EsQueryOptions) (*EsResponse, error) {
	searchUrl := c.EsUrl + "/_search?pretty=true"

	jsonQuery, err := json.Marshal(buildQuery(queryOpts))
	if err != nil {
		return nil, err
	}

	if queryOpts.Show {
		fmt.Printf("ES Query: %+v\n", buildQuery(queryOpts))
		fmt.Printf("--------------------------\n\n")
	}

	req, err := http.NewRequest("POST", searchUrl, bytes.NewBuffer(jsonQuery))
	if err != nil {
		return nil, err
	}

	timeout := c.ConnectTimeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	transport := http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, timeout)
		},
	}
	client := http.Client{Transport: &transport}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var esResp EsResponse
	err = json.Unmarshal(body, &esResp)
	if err != nil {
		return nil, err
	}

	return &esResp, nil
}

func buildQuery(queryOpts EsQueryOptions) map[string]interface{} {
	sort := map[string]map[string]string{
		"@timestamp": map[string]string{
			"order":         "asc",
			"unmapped_type": "long",
		},
	}

	query := map[string]interface{}{
		"filtered": map[string]interface{}{
			"query": map[string]map[string]interface{}{
				"query_string": {
					"query":            string(queryOpts.Query),
					"analyze_wildcard": string("true"),
				},
			},
			"filter": map[string]map[string]map[string]interface{}{
				"range": {
					"@timestamp": {
						"gte": queryOpts.StartTime,
						"lte": queryOpts.EndTime,
					},
				},
			},
		},
	}

	highlight := map[string]interface{}{
		"pre_tags":  []string{"@BEGIN-LOGSEARCH-HIGHLIGHT@"},
		"post_tags": []string{"@END-LOGSEARCH-HIGHLIGHT@"},
		"fields": map[string]interface{}{
			"*": map[string]interface{}{
				"force_source":        "true",
				"fragment_size":       32000,
				"number_of_fragments": 100,
			},
		},
	}

	return map[string]interface{}{
		"size":      queryOpts.NumResults,
		"sort":      sort,
		"query":     query,
		"highlight": highlight,
	}
}
