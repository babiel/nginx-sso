package main

import (
	"net/http"
	"strings"

	"github.com/GeertJohan/yubigo"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

func init() {
	registerMFAProvider(&mfaYubikey{})
}

type mfaYubikey struct {
	ClientID  string `yaml:"client_id"`
	SecretKey string `yaml:"secret_key"`
}

// ProviderID needs to return an unique string to identify
// this special MFA provider
func (m mfaYubikey) ProviderID() (id string) { return "yubikey" }

// Configure loads the configuration for the Authenticator from the
// global config.yaml file which is passed as a byte-slice.
// If no configuration for the Authenticator is supplied the function
// needs to return the errProviderUnconfigured
func (m *mfaYubikey) Configure(yamlSource []byte) (err error) {
	envelope := struct {
		MFA struct {
			Yubikey *mfaYubikey `yaml:"yubikey"`
		} `yaml:"mfa"`
	}{}

	if err := yaml.Unmarshal(yamlSource, &envelope); err != nil {
		return err
	}

	if envelope.MFA.Yubikey == nil {
		return errProviderUnconfigured
	}

	m.ClientID = envelope.MFA.Yubikey.ClientID
	m.SecretKey = envelope.MFA.Yubikey.SecretKey

	return nil
}

// ValidateMFA takes the user from the login cookie and performs a
// validation against the provided MFA configuration for this user
func (m mfaYubikey) ValidateMFA(res http.ResponseWriter, r *http.Request, user string, mfaCfgs []mfaConfig) error {
	var keyInput string

	yubiAuth, err := yubigo.NewYubiAuth(m.ClientID, m.SecretKey)
	if err != nil {
		return errors.Wrap(err, "Unable to create Yubikey client")
	}

	for _, c := range mfaCfgs {
		if c.Provider != m.ProviderID() {
			continue
		}

		for key, values := range r.Form {
			if strings.HasSuffix(key, mfaLoginFieldName) && strings.HasPrefix(values[0], c.AttributeString("device")) {
				keyInput = values[0]
			}
		}

		if keyInput == "" {
			continue
		}

		_, ok, err := yubiAuth.Verify(keyInput)
		if err != nil && !strings.Contains(err.Error(), "OTP has wrong length.") {
			return errors.Wrap(err, "OTP verification failed")
		}

		if ok {
			return nil
		}
	}

	// Not a valid authentication
	return errNoValidUserFound
}
