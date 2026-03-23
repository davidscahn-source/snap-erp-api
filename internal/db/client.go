package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type Client struct {
	URL        string
	ServiceKey string
	httpClient *http.Client
}

var Default *Client

func Init() {
	Default = &Client{
		URL:        os.Getenv("SUPABASE_URL"),
		ServiceKey: os.Getenv("SUPABASE_SECRET_KEY"),
		httpClient: &http.Client{},
	}
}

func (c *Client) headers() map[string]string {
	return map[string]string{
		"apikey":        c.ServiceKey,
		"Authorization": "Bearer " + c.ServiceKey,
		"Content-Type":  "application/json",
		"Prefer":        "return=representation",
	}
}

func (c *Client) Select(table, query string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/v1/%s?%s", c.URL, table, query)
	req, _ := http.NewRequest("GET", url, nil)
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, err := c.httpClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 { return nil, fmt.Errorf("Supabase %d: %s", resp.StatusCode, string(body)) }
	var result []map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

func (c *Client) SelectOne(table, query string) (map[string]interface{}, error) {
	rows, err := c.Select(table, query+"&limit=1")
	if err != nil || len(rows) == 0 { return nil, err }
	return rows[0], nil
}

func (c *Client) Insert(table string, data map[string]interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/v1/%s", c.URL, table)
	body, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, err := c.httpClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 { return nil, fmt.Errorf("Insert %d: %s", resp.StatusCode, string(respBody)) }
	var results []map[string]interface{}
	json.Unmarshal(respBody, &results)
	if len(results) > 0 { return results[0], nil }
	return nil, nil
}

func (c *Client) Update(table, filter string, data map[string]interface{}) error {
	url := fmt.Sprintf("%s/rest/v1/%s?%s", c.URL, table, filter)
	body, _ := json.Marshal(data)
	req, _ := http.NewRequest("PATCH", url, bytes.NewBuffer(body))
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, err := c.httpClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Update %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
