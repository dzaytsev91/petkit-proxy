package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

const (
	targetURL      = "http://api.eu-pet.com"
	petkitHost     = "api.eu-pet.com"
	proxyPort      = ":8080"
	specialPath    = "/6/t4/dev_device_info"
	specialPath2   = "/6/t3/dev_signup"
	specialPath3   = "/6/t3/dev_device_info"
	serverInfoPath = "/6/t3/dev_serverinfo"
)

type Response struct {
	Result Result `json:"result"`
}

type Result struct {
	IPServers  []string `json:"ipServers"`
	APIServers []string `json:"apiServers"`
	NextTick   int      `json:"nextTick"`
	Linked     int      `json:"linked"`
}

func logRequest(r *http.Request) {
	log.Printf(">>> Request: %s %s %s", r.Method, r.URL.String(), r.Proto)
	for name, values := range r.Header {
		for _, value := range values {
			log.Printf(">>> Header: %s: %s", name, value)
		}
	}
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf(">>> Error reading request body: %v", err)
		} else {
			log.Printf(">>> Body: %s", string(body))
			r.Body = io.NopCloser(bytes.NewBuffer(body))
		}
	}
}

func logResponse(resp *http.Response) {
	if resp != nil {
		log.Printf("<<< Response: %s", resp.Status)
		for name, values := range resp.Header {
			for _, value := range values {
				log.Printf("<<< Header: %s: %s", name, value)
			}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("<<< Error reading response body: %v", err)
		} else {
			log.Printf("<<< Body: %s", string(body))
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
		}
	}
}

func modifyResponse(resp *http.Response) error {
	if resp.Request.URL.Path != specialPath && resp.Request.URL.Path != serverInfoPath && resp.Request.URL.Path != specialPath2 && resp.Request.URL.Path != specialPath3 {
		return nil
	}

	if resp.Request.URL.Path == serverInfoPath {
		response := Response{
			Result: Result{
				IPServers:  []string{"http://85.192.37.145:80/6/"},
				APIServers: []string{"http://api.eu-pet.com/6/"},
				NextTick:   3600,
				Linked:     0,
			},
		}

		resp.Header.Set("Content-Type", "application/json")
		modifiedBody, err := json.Marshal(response)
		if err != nil {
			return err
		}

		resp.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
		resp.ContentLength = int64(len(modifiedBody))
		resp.Header.Set("Content-Length", strconv.Itoa(len(modifiedBody)))
		logResponse(resp)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	bodyErr := resp.Body.Close()
	if bodyErr != nil {
		return bodyErr
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("JSON parse error: %v", err)
		return nil
	}

	if result, ok := data["result"].(map[string]interface{}); ok {
		if settings, ok := result["settings"].(map[string]interface{}); ok {
			if autowork, exists := settings["autoWork"].(float64); exists {
				log.Printf("Modifying autowork from %.0f to 1", autowork)
				settings["autoWork"] = 1
			}
		}
	}

	modifiedBody, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
	resp.ContentLength = int64(len(modifiedBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(modifiedBody)))
	logResponse(resp)
	return nil
}

func NewReverseProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		logRequest(req)
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		logResponse(resp)
		return modifyResponse(resp)
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		rw.WriteHeader(http.StatusBadGateway)
		_, _ = rw.Write([]byte("Proxy error: " + err.Error()))
	}

	return proxy
}

func proxyHandler(proxy http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != petkitHost {
			log.Printf("Rejected request for host: %s", r.Host)
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("403 - Host not allowed"))
			return
		}
		log.Printf("Proxying request for host: %s", r.Host)
		proxy.ServeHTTP(w, r)
		return
	})
}

func main() {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatal(err)
	}
	proxy := NewReverseProxy(target)
	handler := proxyHandler(proxy)
	log.Printf("Starting proxy server on %s", proxyPort)
	log.Fatal(http.ListenAndServe(proxyPort, handler))
}
