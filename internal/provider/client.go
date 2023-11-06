package provider

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

var aapSuccessCodes = []int{200, 201, 202, 204}

// Client -
type AAPClient struct {
	HostURL            string
	Username           *string
	Password           *string
	InsecureSkipVerify bool
}

// AAP group
type AapGroup struct {
	Id          int64    `json:"id"`
	Inventory   int64    `json:"inventory"`
	Name        string   `json:"name"`
	Children    []string `json:"children"`
	Description string   `json:"description"`
	Variables   string   `json:"variables"`
}

// AAP host
type AapHost struct {
	Id          int64    `json:"id"`
	Inventory   int64    `json:"inventory"`
	Name        string   `json:"name"`
	Groups      []string `json:"groups"`
	Description string   `json:"description"`
	Variables   string   `json:"variables"`
}

// AAP inventory
type AapInventory struct {
	Id           int64  `json:"id"`
	Organization int64  `json:"organization"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Variables    string `json:"variables"`
}

// ansible host
type AnsibleHost struct {
	Name      string            `json:"name"`
	Groups    []string          `json:"groups"`
	Variables map[string]string `json:"variables"`
}

// ansible host list
type AnsibleHostList struct {
	Hosts []AnsibleHost `json:"hosts"`
}

type DisassociateRequest struct {
	Id           int64 `json:"id"`
	Disassociate bool  `json:"disassociate"`
}

type PagedGroupsResponse struct {
	Count    int64      `json:"count"`
	Next     string     `json:"next"`
	Previous string     `json:"previous"`
	Results  []AapGroup `json:"results"`
}

type PagedHostsResponse struct {
	Count    int64     `json:"count"`
	Next     string    `json:"next"`
	Previous string    `json:"previous"`
	Results  []AapHost `json:"results"`
}

// NewClient -
func NewClient(host string, username *string, password *string, insecure_skip_verify bool) (*AAPClient, error) {
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}

	client := AAPClient{
		HostURL:            host,
		Username:           username,
		Password:           password,
		InsecureSkipVerify: insecure_skip_verify,
	}

	return &client, nil
}

func (c *AAPClient) MakeRequest(method string, endpoint string, requestBody io.Reader) ([]byte, error) {
	req, _ := http.NewRequest(method, endpoint, requestBody)

	if c.Username != nil && c.Password != nil {
		req.SetBasicAuth(*c.Username, *c.Password)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.InsecureSkipVerify},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(aapSuccessCodes, resp.StatusCode) {
		return nil, fmt.Errorf("status: %d, body: %s", resp.StatusCode, responseBody)
	}

	return responseBody, nil
}

func (c *AAPClient) AddChildToGroup(groupId string, childGroupId int64) error {
	requestBody := map[string]int64{"id": childGroupId}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(requestBody)
	if err != nil {
		return err
	}
	_, err = c.MakeRequest("POST", c.HostURL+"api/v2/groups/"+groupId+"/children/", &buf)
	if err != nil {
		return err
	}

	return nil
}

func (c *AAPClient) AddGroupToHost(hostId string, groupId int64) error {
	requestBody := map[string]int64{"id": groupId}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(requestBody)
	if err != nil {
		return err
	}
	_, err = c.MakeRequest("POST", c.HostURL+"api/v2/hosts/"+hostId+"/groups/", &buf)
	if err != nil {
		return err
	}

	return nil
}

func (c *AAPClient) CreateGroup(requestBody io.Reader) (*AapGroup, error) {

	response, err := c.MakeRequest("POST", c.HostURL+"api/v2/groups/", requestBody)
	if err != nil {
		return nil, err
	}

	return ParseGroupResponse(response)
}

func (c *AAPClient) CreateHost(requestBody io.Reader) (*AapHost, error) {

	response, err := c.MakeRequest("POST", c.HostURL+"api/v2/hosts/", requestBody)
	if err != nil {
		return nil, err
	}

	return ParseHostResponse(response)
}

func (c *AAPClient) CreateInventory(requestBody io.Reader) (*AapInventory, error) {

	response, err := c.MakeRequest("POST", c.HostURL+"api/v2/inventories/", requestBody)
	if err != nil {
		return nil, err
	}

	return ParseInventoryResponse(response)
}

func (c *AAPClient) DeleteGroup(groupId string) error {
	_, err := c.MakeRequest("DELETE", c.HostURL+"api/v2/groups/"+groupId+"/", nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *AAPClient) DeleteHost(hostId string) error {
	_, err := c.MakeRequest("DELETE", c.HostURL+"api/v2/hosts/"+hostId+"/", nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *AAPClient) DeleteInventory(inventoryId string) error {
	_, err := c.MakeRequest("DELETE", c.HostURL+"api/v2/inventories/"+inventoryId+"/", nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *AAPClient) GetGroup(groupId string) (*AapGroup, error) {
	response, err := c.MakeRequest("GET", c.HostURL+"api/v2/groups/"+groupId+"/", nil)
	if err != nil {
		return nil, err
	}

	return ParseGroupResponse(response)
}

func (c *AAPClient) GetGroupChildren(groupId string) ([]AapGroup, error) {

	response, err := c.MakeRequest("GET", c.HostURL+"api/v2/groups/"+groupId+"/children", nil)
	if err != nil {
		return nil, err
	}

	return ParsePagedGroupsResponse(response)
}

func (c *AAPClient) GetHostGroups(hostId string) ([]AapGroup, error) {

	response, err := c.MakeRequest("GET", c.HostURL+"api/v2/hosts/"+hostId+"/groups", nil)
	if err != nil {
		return nil, err
	}

	return ParsePagedGroupsResponse(response)
}

func (c *AAPClient) GetHosts(stateId string) (*AnsibleHostList, error) {

	response, err := c.MakeRequest("GET", c.HostURL+"api/v2/state/"+stateId+"/", nil)
	if err != nil {
		return nil, err
	}

	return GetAnsibleHost(response)
}

func (c *AAPClient) GetInventory(inventoryId string) (*AapInventory, error) {

	response, err := c.MakeRequest("GET", c.HostURL+"api/v2/inventories/"+inventoryId+"/", nil)
	if err != nil {
		return nil, err
	}

	return ParseInventoryResponse(response)
}

func (c *AAPClient) GetInventoryGroups(inventoryId string) ([]AapGroup, error) {

	response, err := c.MakeRequest("GET", c.HostURL+"api/v2/inventories/"+inventoryId+"/groups/", nil)
	if err != nil {
		return nil, err
	}

	return ParsePagedGroupsResponse(response)
}

func (c *AAPClient) GetInventoryHosts(inventoryId string) ([]AapHost, error) {

	response, err := c.MakeRequest("GET", c.HostURL+"api/v2/inventories/"+inventoryId+"/hosts/", nil)
	if err != nil {
		return nil, err
	}

	return ParsePagedHostsResponse(response)
}

func (c *AAPClient) RemoveChildFromGroup(groupId string, childGroupId int64) error {
	requestBody := DisassociateRequest{Id: childGroupId, Disassociate: true}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(requestBody)
	if err != nil {
		return err
	}
	_, err = c.MakeRequest("POST", c.HostURL+"api/v2/groups/"+groupId+"/children/", &buf)
	if err != nil {
		return err
	}

	return nil
}

func (c *AAPClient) RemoveGroupFromHost(hostId string, groupId int64) error {
	requestBody := DisassociateRequest{Id: groupId, Disassociate: true}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(requestBody)
	if err != nil {
		return err
	}
	_, err = c.MakeRequest("POST", c.HostURL+"api/v2/hosts/"+hostId+"/groups/", &buf)
	if err != nil {
		return err
	}

	return nil
}

func (c *AAPClient) UpdateGroup(groupId string, requestBody io.Reader) (*AapGroup, error) {

	response, err := c.MakeRequest("PUT", c.HostURL+"api/v2/groups/"+groupId+"/", requestBody)
	if err != nil {
		return nil, err
	}

	return ParseGroupResponse(response)
}

func (c *AAPClient) UpdateHost(hostId string, requestBody io.Reader) (*AapHost, error) {

	response, err := c.MakeRequest("PUT", c.HostURL+"api/v2/hosts/"+hostId+"/", requestBody)
	if err != nil {
		return nil, err
	}

	return ParseHostResponse(response)
}

func (c *AAPClient) UpdateInventory(inventoryId string, requestBody io.Reader) (*AapInventory, error) {

	response, err := c.MakeRequest("PUT", c.HostURL+"api/v2/inventories/"+inventoryId+"/", requestBody)
	if err != nil {
		return nil, err
	}
	return ParseInventoryResponse(response)
}

// Parse responses

func GetAnsibleHost(body []byte) (*AnsibleHostList, error) {

	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	var hosts AnsibleHostList
	resources, ok := result["resources"].([]interface{})
	if ok {
		for _, resource := range resources {
			resource_obj := resource.(map[string]interface{})
			resource_type, ok := resource_obj["type"]
			if ok && resource_type == "ansible_host" {
				instances, ok := resource_obj["instances"].([]interface{})
				if ok {
					for _, instance := range instances {
						attributes, ok := instance.(map[string]interface{})["attributes"].(map[string]interface{})
						if ok {
							name := attributes["name"].(string)
							var groups []string
							for _, group := range attributes["groups"].([]interface{}) {
								groups = append(groups, group.(string))
							}
							variables := make(map[string]string)
							for key, value := range attributes["variables"].(map[string]interface{}) {
								variables[key] = value.(string)
							}
							hosts.Hosts = append(hosts.Hosts, AnsibleHost{
								Name:      name,
								Groups:    groups,
								Variables: variables,
							})
						}
					}
				}
			}
		}
	}
	return &hosts, nil
}

func ParseGroupResponse(body []byte) (*AapGroup, error) {

	var result AapGroup

	err := json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func ParseHostResponse(body []byte) (*AapHost, error) {

	var result AapHost

	err := json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func ParseInventoryResponse(body []byte) (*AapInventory, error) {

	var result AapInventory

	err := json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func ParsePagedGroupsResponse(body []byte) ([]AapGroup, error) {

	var groupsResponse PagedGroupsResponse
	err := json.Unmarshal(body, &groupsResponse)
	if err != nil {
		return nil, err
	}

	return groupsResponse.Results, nil // TODO: Handling paged responses, currently only returning first page
}

func ParsePagedHostsResponse(body []byte) ([]AapHost, error) {

	var hostsResponse PagedHostsResponse
	err := json.Unmarshal(body, &hostsResponse)
	if err != nil {
		return nil, err
	}

	return hostsResponse.Results, nil // TODO: Handling paged responses, currently only returning first page
}
