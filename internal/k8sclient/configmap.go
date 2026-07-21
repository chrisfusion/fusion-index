package k8sclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type configMap struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   configMapMetadata `json:"metadata"`
	Data       map[string]string `json:"data"`
}

type configMapMetadata struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

func configMapURL(namespace, name string) string {
	return fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps/%s", APIBase, namespace, name)
}

// GetConfigMapData fetches a ConfigMap's data map and resourceVersion (needed for a
// subsequent UpdateConfigMapData call). found is false (with a nil error) when the
// ConfigMap does not exist yet.
func GetConfigMapData(client *http.Client, token, namespace, name string) (data map[string]string, resourceVersion string, found bool, err error) {
	req, err := http.NewRequest(http.MethodGet, configMapURL(namespace, name), nil)
	if err != nil {
		return nil, "", false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", false, fmt.Errorf("get configmap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", false, fmt.Errorf("get configmap returned %d: %s", resp.StatusCode, string(b))
	}

	var cm configMap
	if err := json.NewDecoder(resp.Body).Decode(&cm); err != nil {
		return nil, "", false, fmt.Errorf("decode configmap: %w", err)
	}
	return cm.Data, cm.Metadata.ResourceVersion, true, nil
}

// CreateConfigMapData creates a new ConfigMap with the given data.
func CreateConfigMapData(client *http.Client, token, namespace, name string, data map[string]string) error {
	cm := configMap{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   configMapMetadata{Name: name, Namespace: namespace},
		Data:       data,
	}
	body, err := json.Marshal(cm)
	if err != nil {
		return fmt.Errorf("marshal configmap: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps", APIBase, namespace)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("create configmap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create configmap returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// UpdateConfigMapData replaces an existing ConfigMap's data. resourceVersion must come
// from a prior GetConfigMapData call so the API server can detect concurrent writes.
func UpdateConfigMapData(client *http.Client, token, namespace, name, resourceVersion string, data map[string]string) error {
	cm := configMap{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   configMapMetadata{Name: name, Namespace: namespace, ResourceVersion: resourceVersion},
		Data:       data,
	}
	body, err := json.Marshal(cm)
	if err != nil {
		return fmt.Errorf("marshal configmap: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, configMapURL(namespace, name), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("update configmap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update configmap returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
