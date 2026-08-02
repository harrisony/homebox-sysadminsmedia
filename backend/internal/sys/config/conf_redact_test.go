package config

import (
	"encoding/json"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/ardanlabs/conf/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sentinel = redactedValue

func Test_AuthConfig_RedactsAPIKeyPepper(t *testing.T) {
	t.Parallel()

	c := AuthConfig{APIKeyPepper: "super-secret-pepper"}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.NotContains(t, string(out), "super-secret-pepper")
	assert.Contains(t, string(out), sentinel)
}

func Test_AuthConfig_EmptyAPIKeyPepperStaysEmpty(t *testing.T) {
	t.Parallel()

	c := AuthConfig{}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.NotContains(t, string(out), sentinel)
}

func Test_OIDCConf_RedactsClientSecret(t *testing.T) {
	t.Parallel()

	c := OIDCConf{ClientID: "public-client-id", ClientSecret: "shh"}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.Contains(t, string(out), "public-client-id")
	assert.NotContains(t, string(out), `"shh"`)
	assert.Contains(t, string(out), sentinel)
}

func Test_MailerConf_RedactsPassword(t *testing.T) {
	t.Parallel()

	c := MailerConf{Host: "smtp.example.com", Username: "u", Password: "pw", From: "f"}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.NotContains(t, string(out), `"pw"`)
	assert.Contains(t, string(out), `"u"`)
	assert.Contains(t, string(out), sentinel)
}

func Test_Database_RedactsPasswordAndPubSubCreds(t *testing.T) {
	t.Parallel()

	c := Database{
		Driver:           "postgres",
		Username:         "homebox",
		Password:         "dbpass",
		Host:             "db",
		Port:             "5432",
		Database:         "homebox",
		PubSubConnString: "postgres://pubuser:pubpass@db:5432/homebox?sslmode=disable",
	}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	s := string(out)
	assert.NotContains(t, s, "dbpass")
	assert.NotContains(t, s, "pubpass")
	assert.Contains(t, s, "pubuser", "username portion should remain visible")
	assert.Contains(t, s, sentinel)
}

func Test_Database_LeavesUncredentialedPubSubAlone(t *testing.T) {
	t.Parallel()

	c := Database{PubSubConnString: "mem://{{ .Topic }}"}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.Contains(t, string(out), "mem://")
}

func Test_Storage_RedactsConnStringUserinfo(t *testing.T) {
	t.Parallel()

	c := Storage{ConnString: "s3://AKIA:secret@bucket.example.com/path"}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.NotContains(t, string(out), "secret@bucket")
	assert.Contains(t, string(out), "REDACTED")
}

func Test_BarcodeAPIConf_RedactsToken(t *testing.T) {
	t.Parallel()

	c := BarcodeAPIConf{TokenBarcodespider: "token-xyz", OpenFoodFactsContact: "contact@example.com"}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.NotContains(t, string(out), "token-xyz")
	assert.Contains(t, string(out), "contact@example.com")
	assert.Contains(t, string(out), sentinel)
}

func Test_OTelConfig_RedactsHeaders(t *testing.T) {
	t.Parallel()

	c := OTelConfig{Headers: "Authorization=Bearer hunter2,X-Other=val"}

	out, err := json.Marshal(c)
	require.NoError(t, err)

	assert.NotContains(t, string(out), "hunter2")
	assert.Contains(t, string(out), sentinel)
}

func Test_Config_FullMarshalRedactsAllSecrets(t *testing.T) {
	t.Parallel()

	c := &Config{}
	fieldSentinels := setMaskedFields(t, reflect.ValueOf(c).Elem(), "")
	require.NotEmpty(t, fieldSentinels, "expected at least one conf:\"mask\" field on Config")

	out, err := json.MarshalIndent(c, "", "  ")
	require.NoError(t, err)

	for path, value := range fieldSentinels {
		assert.NotContainsf(t, string(out), value, "field %s: sentinel value leaked in JSON output", path)
	}
}

func Test_Config_ConfStringMasksAllSecrets(t *testing.T) {
	t.Parallel()

	c := &Config{}
	fieldSentinels := setMaskedFields(t, reflect.ValueOf(c).Elem(), "")
	require.NotEmpty(t, fieldSentinels, "expected at least one conf:\"mask\" field on Config")

	out, err := conf.String(c)
	require.NoError(t, err)

	for path, value := range fieldSentinels {
		assert.NotContainsf(t, out, value, "field %s: sentinel value leaked in configuration output", path)
	}
}

func setMaskedFields(t *testing.T, v reflect.Value, path string) map[string]string {
	t.Helper()

	fieldSentinels := make(map[string]string)

	typ := v.Type()
	for i := range typ.NumField() {
		field := typ.Field(i)
		fv := v.Field(i)

		fieldPath := field.Name
		if path != "" {
			fieldPath = path + "." + field.Name
		}

		if fv.Kind() == reflect.Struct {
			maps.Copy(fieldSentinels, setMaskedFields(t, fv, fieldPath))
			continue
		}

		if fv.Kind() != reflect.String || !fv.CanSet() || !hasMaskTag(field) {
			continue
		}

		value := "sentinel-" + fieldPath
		if strings.HasSuffix(fieldPath, "ConnString") {
			value = "postgres://user:" + value + "@example.com/database"
		}

		fv.SetString(value)
		fieldSentinels[fieldPath] = fv.String()
	}

	return fieldSentinels
}

func hasMaskTag(field reflect.StructField) bool {
	return strings.Contains(field.Tag.Get("conf"), "mask")
}
