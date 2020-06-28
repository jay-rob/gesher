package proxyvalidatingtype

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/redislabs/gesher/pkg/apis/app/v1alpha1"
)

var (
	logger = zap.Logger()
)

const (
	uid         = "1"
	testOp      = v1beta1.Create
	testDiffOp  = v1beta1.Delete
	testGroup   = "testGroup"
	testVersion = "testVersion"
	testKind    = "testKind"
)

var (
	rule = v1beta1.Rule{
		APIGroups:   []string{testGroup},
		APIVersions: []string{testVersion},
		Resources:   []string{testKind},
	}
)

func TestAnalyzeSame(t *testing.T) {
	proxyTypeData := &ProxyTypeData{}
	customResource := &appv1alpha1.ProxyValidatingType{
		ObjectMeta: metav1.ObjectMeta{UID: uid},
		Spec: appv1alpha1.ProxyValidatingTypeSpec{
			Types: []v1beta1.RuleWithOperations{{
				Operations: []v1beta1.OperationType{testOp},
				Rule:       rule,
			}},
		},
	}

	proxyTypeData = proxyTypeData.Add(customResource)
	webhook := proxyTypeData.GenerateGlobalWebhook()

	observed := &observedState{
		customResource: customResource,
		clusterWebhook: webhook,
	}

	state, err := analyze(observed, logger)
	assert.Nil(t, err)
	assert.False(t, state.update)
}

func TestAnalyzeDifferent(t *testing.T) {
	proxyTypeData := &ProxyTypeData{}
	customResource := &appv1alpha1.ProxyValidatingType{
		ObjectMeta: metav1.ObjectMeta{UID: uid},
		Spec: appv1alpha1.ProxyValidatingTypeSpec{
			Types: []v1beta1.RuleWithOperations{{
				Operations: []v1beta1.OperationType{testOp},
				Rule:       rule,
			}},
		},
	}

	proxyTypeData = proxyTypeData.Add(customResource)
	webhook := proxyTypeData.GenerateGlobalWebhook()
	customResource.Spec.Types[0].Operations[0] = testDiffOp

	observed := &observedState{
		customResource: customResource,
		clusterWebhook: webhook,
	}

	state, err := analyze(observed, logger)
	assert.Nil(t, err)
	assert.True(t, state.update)
}