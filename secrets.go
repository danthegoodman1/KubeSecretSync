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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	k8sClientSet *kubernetes.Clientset

	LastUpdatedAnnotation = "kube-secret-sync-last-updated"
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
		logger.Debug("No secrets found to sync")
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
		logger.Errorf("Error marshaling secret %s/%s to json", secret.Namespace, secret.Name)
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
		logger.Errorf("Error upserting secret %s/%s", secret.Namespace, secret.Name)
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
				err = createNewSecret(ctx, secret)
				if err != nil {
					logger.Error("Error creating new secret")
					return err
				}
				continue
			} else {
				logger.Error("Error getting secret from k8s api")
				return err
			}
		}
		logger.Debugf("Found secret %s/%s, checking if updated", secret.Ns, secret.SecretName)

		// Check if we have the latest update from the DB
		localUpdatedTime, exists := foundSecret.Annotations[LastUpdatedAnnotation]
		if exists && localUpdatedTime < secret.UpdatedAt.UTC().Format(time.RFC3339) {
			logger.Infof("Found an older version of %s/%s, updating", secret.Ns, secret.SecretName)
		} else if !exists {
			logger.Infof("Did not have annotation for %s/%s, replacing secret", secret.Ns, secret.SecretName)
		} else {
			logger.Debugf("Secret %s/%s up to date with DB", secret.Ns, secret.SecretName)
			continue
		}

		err = patchSecret(ctx, secret)
		if err != nil {
			logger.Errorf("Failed to patch secret %s/%s", secret.Ns, secret.SecretName)
			return err
		}

		logger.Infof("Patched %s/%s in %s", secret.Ns, secret.SecretName, time.Since(s))
	}

	return nil
}

func prepareDecryptedSecret(secret corev1.Secret) (corev1.Secret, error) {
	// Decrypt data
	for key, cipherText := range secret.Data {
		logger.Debugf("Decrypting secret %s/%s with key %s", secret.Namespace, secret.Name, key)
		plainText, err := decryptBytes(cipherText, utils.ENCRYPTION_KEY)
		if err != nil {
			logger.Errorf("Error decrypting data with key %s", key)
			return corev1.Secret{}, err
		}

		// Replace with plain text
		secret.Data[key] = plainText
	}

	// Need to drop the resource version
	secret.ResourceVersion = ""

	return secret, nil
}

func createNewSecret(ctx context.Context, secret query.KssSecret) error {
	// Secret not found, create it
	logger.Infof("New secret %s/%s found! Creating...", secret.Ns, secret.SecretName)

	// Bind to secret object
	var newSecret corev1.Secret
	err := secret.Manifest.AssignTo(&newSecret)
	if err != nil {
		logger.Error("Error assigning secret manifest")
		return err
	}

	newSecret, err = prepareDecryptedSecret(newSecret)
	if err != nil {
		logger.Errorf("Error preparing decrypted secret %s/%s", secret.Ns, secret.SecretName)
		return err
	}

	if newSecret.Annotations == nil {
		newSecret.Annotations = map[string]string{}
	}

	newSecret.Annotations[LastUpdatedAnnotation] = secret.UpdatedAt.UTC().Format(time.RFC3339)

	// Create the secret
	_, err = k8sClientSet.CoreV1().Secrets(secret.Ns).Create(ctx, &newSecret, v1.CreateOptions{})
	if err != nil {
		logger.Errorf("Error creating secret %s/%s", secret.Ns, secret.SecretName)
		return err
	}

	return nil
}

func patchSecret(ctx context.Context, secret query.KssSecret) error {
	logger.Infof("Patching secret %s/%s...", secret.Ns, secret.SecretName)

	// Bind to secret object
	var newSecret corev1.Secret
	err := secret.Manifest.AssignTo(&newSecret)
	if err != nil {
		logger.Error("Error assigning secret manifest")
		return err
	}

	newSecret, err = prepareDecryptedSecret(newSecret)
	if err != nil {
		logger.Errorf("Error preparing decrypted secret %s/%s", secret.Ns, secret.SecretName)
		return err
	}

	// Add the latest hash annotation, only include data, annotations, and labels
	finalSecret := corev1.Secret{}
	finalSecret.Name = newSecret.Name
	finalSecret.Namespace = newSecret.Namespace
	finalSecret.Labels = newSecret.GetLabels()
	finalSecret.Annotations = newSecret.GetAnnotations()
	finalSecret.Data = newSecret.Data

	if finalSecret.Annotations == nil {
		finalSecret.Annotations = map[string]string{}
	}

	finalSecret.Annotations[LastUpdatedAnnotation] = secret.UpdatedAt.UTC().Format(time.RFC3339)

	patchBytes, err := json.Marshal(&finalSecret)
	if err != nil {
		logger.Errorf("Error marshaling final secret %s/%s for patch", secret.Ns, secret.SecretName)
		return err
	}

	logger.Debugf("Patching with %s", string(patchBytes))

	_, err = k8sClientSet.CoreV1().Secrets(secret.Ns).Patch(ctx, secret.SecretName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		logger.Errorf("Error patching secret %s/%s", secret.Ns, secret.SecretName)
		return err
	}

	return nil
}
