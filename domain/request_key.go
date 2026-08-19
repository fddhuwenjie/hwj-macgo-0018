package domain

import "fmt"

type RequestKey struct {
	CallerID    string
	NamespaceID string
	Key         string
}

func NewRequestKey(callerID, namespaceID, key string) (RequestKey, error) {
	k := RequestKey{CallerID: callerID, NamespaceID: namespaceID, Key: key}
	if err := ValidateRequestKey(k); err != nil {
		return RequestKey{}, err
	}
	return k, nil
}

func (k RequestKey) Validate() error {
	return ValidateRequestKey(k)
}

func (k RequestKey) String() string {
	return fmt.Sprintf("%s/%s/%s", k.CallerID, k.NamespaceID, k.Key)
}
