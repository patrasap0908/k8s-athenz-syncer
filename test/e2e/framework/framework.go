package framework

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/yahoo/athenz/clients/go/zms"
	athenzClientset "github.com/yahoo/k8s-athenz-syncer/pkg/client/clientset/versioned"
	athenzInformer "github.com/yahoo/k8s-athenz-syncer/pkg/client/informers/externalversions/athenz/v1"
	"github.com/yahoo/k8s-athenz-syncer/pkg/cr"
	"github.com/yahoo/k8s-athenz-syncer/pkg/log"
	"github.com/yahoo/k8s-athenz-syncer/pkg/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type Framework struct {
	K8sClient kubernetes.Interface
	ZMSClient zms.ZMSClient
	CRClient  cr.CRUtil
}

var Global *Framework

// Setup() create necessary clients for tests
func setup() error {
	// config
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	inClusterConfig := flag.Bool("inClusterConfig", true, "Set to true to use in cluster config.")
	key := flag.String("key", "/var/run/athenz/service.key.pem", "Athenz private key file")
	cert := flag.String("cert", "/var/run/athenz/service.cert.pem", "Athenz certificate file")
	zmsURL := flag.String("zms-url", "", "Athenz ZMS API URL")
	logLoc := flag.String("log-location", "/var/log/k8s-athenz-syncer/k8s-athenz-syncer.log", "log location")
	logMode := flag.String("log-mode", "info", "logger mode")
	flag.Parse()

	// init logger
	log.InitLogger(*logLoc, *logMode)

	// if kubeconfig is empty
	if *kubeconfig == "" {
		if *inClusterConfig {
			emptystr := ""
			kubeconfig = &emptystr
		} else {
			if home := util.HomeDir(); home != "" {
				kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
			}
		}
	}
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return err
	}

	// set up k8s client
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error("Failed to create k8s client")
		return err
	}

	// set up athenzdomains client
	athenzClient, err := athenzClientset.NewForConfig(config)
	if err != nil {
		log.Error("Failed to create athenz domains client")
		return err
	}
	// set up cr informer to get athenzdomains resources
	crIndexInformer := athenzInformer.NewAthenzDomainInformer(athenzClient, 0, cache.Indexers{})
	crutil := cr.NewCRUtil(athenzClient, crIndexInformer)

	// set up zms client
	zmsclient, err := setupZMSClient(*key, *cert, *zmsURL)
	if err != nil {
		log.Error("Failed to create zms client")
		return err
	}

	Global = &Framework{
		K8sClient: k8sclient,
		ZMSClient: *zmsclient,
		CRClient:  *crutil,
	}
	return nil
}

// set up zms client, skipping cert reloader
func setupZMSClient(key string, cert string, zmsURL string) (*zms.ZMSClient, error) {
	clientCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("Unable to formulate clientCert from key and cert bytes, error: %v", err)
	}
	config := &tls.Config{}
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0] = clientCert
	transport := &http.Transport{
		TLSClientConfig: config,
	}
	client := zms.NewClient(zmsURL, transport)
	return &client, nil
}

// teardown Framework
func teardown() error {
	Global = nil
	log.Info("e2e teardown successfully")
	return nil
}
