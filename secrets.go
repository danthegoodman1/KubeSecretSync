package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"time"

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

	if len(secrets.Items) == 0 {
		logger.Debug("No secrets found to sync, exiting")
		return nil
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
	}

	return nil
}

func upsertSecret(ctx context.Context, secret corev1.Secret, secretHash [32]byte) error {
	logger.Debugf("Upserting secret %s/%s...", secret.Namespace, secret.Name)
	s := time.Now()
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

	logger.Debugf("Upsert secret %s/%s affected %d rows in %s", secret.Namespace, secret.Name, rows, time.Since(s))

	return nil
}

func tickFollower(ctx context.Context) error {
	logger.Debug("Ticking as follower...")

	// Get all secrets in DB
	s := time.Now()
	conn, err := db.PGPool.Acquire(ctx)
	if err != nil {
		logger.Error("Error acquiring pool connection")
		return err
	}
	defer conn.Release()

	secrets, err := query.New(conn).ListAllSecrets(ctx)
	if err != nil {
		logger.Error("Error listing all secrets")
		return err
	}
	logger.Debugf("Listed %d secrets in %s", len(secrets), time.Since(s))

	// Compare all DB secrets to local
	for _, secret := range secrets {
		logger.Debugf("Checking if secret %s/%s exists", secret.Ns, secret.SecretName)
		foundSecret, err := k8sClientSet.CoreV1().Secrets(secret.Ns).Get(ctx, secret.SecretName, v1.GetOptions{})
		if err != nil {
			if strings.HasSuffix(err.Error(), " not found") {
				// Secret not found, create it
				logger.Infof("New secret %s/%s found! Creating...", secret.Ns, secret.SecretName)

				// Bind to secret object
				var newSecret corev1.Secret
				err := secret.Manifest.AssignTo(&newSecret)
				if err != nil {
					logger.Error("Error assigning secret manifest")
					return err
				}

				// Decrypt data
				for key, cipherText := range newSecret.Data {
					logger.Debugf("Decrypting secret %s/%s with key %s", secret.Ns, secret.SecretName, key)
					plainText, err := decryptBytes(cipherText, utils.ENCRYPTION_KEY)
					if err != nil {
						logger.Errorf("Error decrypting data with key %s", key)
						return err
					}

					// Replace with cipher text
					newSecret.Data[key] = plainText
				}

				// Need to drop the resource version
				newSecret.ResourceVersion = ""

				// Create the secret
				_, err = k8sClientSet.CoreV1().Secrets(secret.Ns).Create(ctx, &newSecret, v1.CreateOptions{})
				if err != nil {
					logger.Errorf("Error creating secret %s/%s", secret.Ns, secret.SecretName)
					return err
				}

				continue
			} else {
				logger.Error("Error getting secret from k8s api")
				return err
			}
		}
		foundSecret = foundSecret
		// If not exists, create secret

		// If exists and updated time is different than annotation, update
	}

	return nil
}
