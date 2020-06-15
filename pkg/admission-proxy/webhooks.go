package admission_proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"

	"k8s.io/api/admission/v1beta1"
	admv1beta1 "k8s.io/api/admissionregistration/v1beta1"
)

// temporary dumb webhook handler
type Webhook map[string]string

const (
	serviceName = "service-name"
	namespace   = "namespace"
	port        = "port"
	path        = "path"
	caBundle    = "ca-bundle"
)

func findWebhooks(request *v1beta1.AdmissionRequest) []Webhook {
	webhook := make(Webhook)

	if webhook[serviceName] = os.Getenv("ADM_SERVICE_NAME"); webhook[serviceName] == "" {
		panic("failed to read env variable ADM_SERVICE_NAME")
	}
	if webhook[namespace] = os.Getenv("ADM_SERVICE_NAMESPACE"); webhook[namespace] == "" {
		panic("failed to read env variable ADM_SERVICE_NAMESPACE")
	}
	if webhook[path] = os.Getenv("ADM_SERVICE_ENDPOINT"); webhook[path] == "" {
		panic("failed to read env variable ADM_SERVICE_ENDPOINT")
	}
	encoded := os.Getenv("ADM_SERVICE_CABUNDLE")
	if encoded == "" {
		panic("failed to read env variable ADM_SERVICE_CABUNDLE")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		panic("failed to base64 decode cabundle")
	}
	webhook[caBundle] = string(data)

	if webhook[port] = os.Getenv("ADM_SERVICE_PORT"); webhook[port] == "" {
		panic("failed to read env variable ADM_SERVICE_PORT")
	}
	if _, err := strconv.Atoi(webhook[port]); err != nil {
		panic(fmt.Errorf("failed to convert port to an int: %v", err))
	}

	return []Webhook{webhook}
}

// code is inspired by k8s.io/apiserver/pkg/admission/plugin/webhook/validating/dispatcher.go
func checkWebhooks(webhooks []Webhook, r *http.Request, body *bytes.Reader) *v1beta1.AdmissionResponse {
	if len(webhooks) == 0 {
		return approved()
	}

	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(webhooks))
	wg.Add(len(webhooks))
	for _, webhook := range webhooks {
		go doWebhook(webhook, wg, r, body, errCh)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for e := range errCh {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return approved()
	}

	if len(errs) > 1 {
		// TODO: merge status errors; until then, just return the first one.
		log.V(3).Info("TODO: merge status errors; until then, just return the first one.")
	}

	return errToAdmissionResponse(errs[0])
}

func doWebhook(webhook Webhook, wg *sync.WaitGroup, r *http.Request, body *bytes.Reader, errCh chan error) {
	defer wg.Done()

	url := fmt.Sprintf("https://%v.%v:%v%v", webhook[serviceName], webhook[namespace], webhook[port], webhook[path])

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(webhook[caBundle]))

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}
	req, _ := http.NewRequestWithContext(context.TODO(), "POST", url, body)
	for k, v := range r.Header {
		for _, s := range v {
			req.Header.Add(k, s)
		}
	}

	resp, err := client.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	failurePolicy := admv1beta1.Fail
	err = toFailure("webhook", resp, err, failurePolicy)

	errCh <- err
}

func toFailure(name string, resp *http.Response, httpErr error, failurePolicy admv1beta1.FailurePolicyType) error {
	log.V(2).Info(fmt.Sprintf("toFailure: %v: httpErr = %v", name, httpErr))
	if httpErr != nil {
		return errToFailure(name, fmt.Errorf("http error: %v", httpErr), failurePolicy)
	}

	if resp.Body == nil {
		return errToFailure(name, errors.New("response body is nil"), failurePolicy)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errToFailure(name, fmt.Errorf("ReadAll failed: %v", err), failurePolicy)
	}

	log.V(2).Info(fmt.Sprintf("toFailure: resp.Body = %v", string(data)))

	var responseAdmissionReview v1beta1.AdmissionReview
	err = json.Unmarshal(data, &responseAdmissionReview)
	if err != nil {
		return errToFailure(name, fmt.Errorf("json unmarshall failed: %v", err), failurePolicy)
	}

	log.V(2).Info(fmt.Sprintf("toFailure: unmarshalled response = %+v\n", responseAdmissionReview))

	if !responseAdmissionReview.Response.Allowed {
		return fmt.Errorf("proxied webhook %v denied the request: %v", name, responseAdmissionReview.Response.Result.Message)
	}

	log.V(2).Info("toFailure: passed all test")

	return nil
}

func errToFailure(name string, err error, failurePolicy admv1beta1.FailurePolicyType) error {
	switch failurePolicy {
	case admv1beta1.Fail:
		log.V(1).Info(fmt.Sprintf("err = %v and FailurePolicy == Fail", err))
		return fmt.Errorf("proxied webhook %v failed: %v", name, err)
	default:
		log.V(1).Info(fmt.Sprintf("err = %v and FailurePolicy == Ignore", err))
		return nil
	}
}