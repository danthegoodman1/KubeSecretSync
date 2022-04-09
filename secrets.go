package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/danthegoodman1/KubeSecretSync/db"
	"github.com/danthegoodman1/KubeSecretSync/query"
	"github.com/danthegoodman1/KubeSecretSync/utils"
	"github.com/jackc/pgtype"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	k8sClientSet *kubernetes.Clientset
)

// Setup the k8s client inside the cluster
func initK8sClient() (err error) {
	logger.Debug("Initializing k8s client...")
	kubeContextPath := os.Getenv("KUBE_CONTEXT_PATH")
	if kubeContextPath == "" {
		logger.Debug("Loading k8s in cluster config")
		restconfig, err := rest.InClusterConfig()
		if err != nil {
			logger.Error("Error getting in cluster config")
			return err
		}

		k8sClientSet, err = kubernetes.NewForConfig(restconfig)
		if err != nil {
			logger.Error("Error initializing k8s client set")
			return err
		}
	} else {
		logger.Debug("Loading KUBE_CONTEXT_PATH config")
		config, err := clientcmd.BuildConfigFromFlags("", kubeContextPath)
		if err != nil {
			logger.Errorf("Error building config from file %s", kubeContextPath)
			return err
		}
		k8sClientSet, err = kubernetes.NewForConfig(config)
		if err != nil {
			logger.Error("Error getting in cluster config")
			return err
		}
	}

	return nil
}

// Queries k8s API for secrets
func tickLeader(ctx context.Context) error {
	logger.Debug("Ticking as leader...")
	// "" namespace lists all namespaces
	secrets, err := k8sClientSet.CoreV1().Secrets("").List(ctx, v1.ListOptions{
		LabelSelector: "kube-secret-sync=true",
	})
	if err != nil {
		logger.Error("Error listing secrets")
		return err
	}

	for _, secret := range secrets.Items {
		logger.Debugf("Got secret %s/%s\n", secret.Namespace, secret.Name)
		// Get hash with original data, since nonce will change the encrypted results between runs
		jsonManifest, err := json.Marshal(&secret)
		if err != nil {
			logger.Error("Error marshaling secret %s/%s to json", secret.Namespace, secret.Name)
			return err
		}
		manifestHash := sha256.Sum256(jsonManifest)

		for key, val := range secret.Data {
			// Encrypt the data values
			logger.Debugf("Encrypting secret %s/%s with key %s", secret.Namespace, secret.Name, key)
			cipherText, err := encryptBytes(val, utils.ENCRYPTION_KEY)
			if err != nil {
				logger.Errorf("Error encrypting data with key %s", key)
				return err
			}

			// Replace with cipher text
			secret.Data[key] = cipherText

			// Drop last applied configuration, since kubectl can include base64 encoded secrets in here
			delete(secret.ObjectMeta.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
		}
		err = upsertSecret(ctx, secret, manifestHash)
		if err != nil {
			logger.Errorf("Error upserting secret %s/%s", secret.Namespace, secret.Name)
			return err
		}

		// rows, err := query.New(conn).UpsertSecret(ctx, query.UpsertSecretParams{
		// 	Ns: secret.Namespace,
		// 	SecretName: secret.Name,
		// 	Manifest: ,
		// })
	}

	return nil
}

func upsertSecret(ctx context.Context, secret corev1.Secret, secretHash [32]byte) error {
	logger.Debugf("Upserting secret %s/%s...", secret.Namespace, secret.Name)
	conn, err := db.PGPool.Acquire(ctx)
	if err != nil {
		logger.Error("Error acquiring pool connection")
		return err
	}
	defer conn.Release()

	jsonManifest, err := json.Marshal(&secret)
	if err != nil {
		logger.Error("Error marshaling secret %s/%s to json", secret.Namespace, secret.Name)
		return err
	}

	jsonString := string(jsonManifest)
	var v pgtype.JSON
	v.Set(&jsonString)

	rows, err := query.New(conn).UpsertSecret(ctx, query.UpsertSecretParams{
		Ns:           secret.Namespace,
		SecretName:   secret.Name,
		Manifest:     v,
		ManifestHash: hex.EncodeToString(secretHash[:]),
	})
	if err != nil {
		logger.Error("Error upserting secret %s/%s", secret.Namespace, secret.Name)
		return err
	}

	logger.Debugf("Upsert secret %s/%s affected %d rows", secret.Namespace, secret.Name, rows)

	return nil
}
