// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"bytes"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// keychainCredentialType is the fixed "type" discriminator this package's
// resource/datasource always send/expect from keychaincredential.* — probed
// live: the field is immutable ("Please note that you can't change `type`",
// per keychaincredential.update's own description) and this resource never
// manages the other value (SSH_KEY_PAIR — see the sibling
// keychain_ssh_keypair package).
const keychainCredentialType = "SSH_CREDENTIALS"

// KeychainSSHConnectionModel is the Terraform state/plan model for
// truenas_keychain_ssh_connection.
type KeychainSSHConnectionModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	PrivateKeyID   types.Int64  `tfsdk:"private_key_id"`
	RemoteHostKey  types.String `tfsdk:"remote_host_key"`
	ConnectTimeout types.Int64  `tfsdk:"connect_timeout"`
}

// KeychainSSHConnectionDataSourceModel is the read-only lookup model for the
// truenas_keychain_ssh_connection data source, looked up by "name"
// (keychaincredential names must be unique across the whole keychain, not
// scoped per type).
type KeychainSSHConnectionDataSourceModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	PrivateKeyID   types.Int64  `tfsdk:"private_key_id"`
	RemoteHostKey  types.String `tfsdk:"remote_host_key"`
	ConnectTimeout types.Int64  `tfsdk:"connect_timeout"`
}

// keychainSSHConnectionAttributesAPI mirrors keychaincredential.*'s
// "attributes" object for type=SSH_CREDENTIALS. Probed live against
// TrueNAS 25.10: "private_key" on the wire is the *numeric id* of a
// SSH_KEY_PAIR keychaincredential, not key material — renamed PrivateKeyID
// here (and in the Terraform-facing schema, "private_key_id") to avoid
// confusion with keychain_ssh_keypair's like-named field, which holds the
// actual PEM key.
type keychainSSHConnectionAttributesAPI struct {
	Host           string `json:"host"`
	Port           int64  `json:"port"`
	Username       string `json:"username"`
	PrivateKey     int64  `json:"private_key"`
	RemoteHostKey  string `json:"remote_host_key"`
	ConnectTimeout int64  `json:"connect_timeout"`

	// Redacted is set (not an API field) when the middleware returned the whole
	// attributes value as the string "********" instead of an object — see
	// UnmarshalJSON. (#57)
	Redacted bool `json:"-"`
}

// UnmarshalJSON tolerates the redacted form of "attributes". For a session
// without the KEYCHAIN_CREDENTIAL write role, TrueNAS redacts the entire
// attributes object to the JSON string "********" rather than an object;
// unmarshaling that into this struct would otherwise fail with an opaque
// "cannot unmarshal string into ... attributes". Detect a JSON string where an
// object is expected, flag it, and let callers surface an actionable
// diagnostic. (#57)
func (a *keychainSSHConnectionAttributesAPI) UnmarshalJSON(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '"' {
		a.Redacted = true
		return nil
	}
	type alias keychainSSHConnectionAttributesAPI
	var x alias
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	*a = keychainSSHConnectionAttributesAPI(x)
	return nil
}

// redactedAttributesError is the diagnostic returned when a read sees redacted
// attributes: the session cannot reveal credential attributes, so the read
// cannot proceed. (#57)
func redactedAttributesError() diag.Diagnostics {
	var d diag.Diagnostics
	d.AddError(
		"Keychain credential attributes are redacted",
		"TrueNAS returned this credential's \"attributes\" as \"********\": the session used for the "+
			"read lacks the KEYCHAIN_CREDENTIAL write role, which the middleware requires to reveal "+
			"credential attributes. Re-run with a full-admin session, or an API key whose user holds "+
			"KEYCHAIN_CREDENTIAL_WRITE — KEYCHAIN_CREDENTIAL_READ is not sufficient, only the write role unredacts.",
	)
	return d
}

// keychainSSHConnectionAPI mirrors the JSON object returned by
// keychaincredential.create/update/get_instance/query for a
// SSH_CREDENTIALS entry.
type keychainSSHConnectionAPI struct {
	ID         int64                              `json:"id"`
	Name       string                             `json:"name"`
	Type       string                             `json:"type"`
	Attributes keychainSSHConnectionAttributesAPI `json:"attributes"`
}

// attributesPayload builds the "attributes" object sent on both create and
// update. keychaincredential.update requires the complete attributes value
// (probed live and stated in the API's own create/update descriptions),
// never a partial merge, so both callers always include every field.
func (m *KeychainSSHConnectionModel) attributesPayload() map[string]any {
	return map[string]any{
		"host":            m.Host.ValueString(),
		"port":            m.Port.ValueInt64(),
		"username":        m.Username.ValueString(),
		"private_key":     m.PrivateKeyID.ValueInt64(),
		"remote_host_key": m.RemoteHostKey.ValueString(),
		"connect_timeout": m.ConnectTimeout.ValueInt64(),
	}
}

// createPayload builds the map expected by keychaincredential.create for
// type=SSH_CREDENTIALS.
func (m *KeychainSSHConnectionModel) createPayload() map[string]any {
	return map[string]any{
		"name":       m.Name.ValueString(),
		"type":       keychainCredentialType,
		"attributes": m.attributesPayload(),
	}
}

// updatePayload builds the second arg of keychaincredential.update.
func (m *KeychainSSHConnectionModel) updatePayload() map[string]any {
	return map[string]any{
		"name":       m.Name.ValueString(),
		"attributes": m.attributesPayload(),
	}
}

// responseToModel maps a keychainSSHConnectionAPI response onto a
// KeychainSSHConnectionModel.
func responseToModel(api *keychainSSHConnectionAPI, m *KeychainSSHConnectionModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if api.Attributes.Redacted {
		return redactedAttributesError()
	}

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Host = types.StringValue(api.Attributes.Host)
	m.Port = types.Int64Value(api.Attributes.Port)
	m.Username = types.StringValue(api.Attributes.Username)
	m.PrivateKeyID = types.Int64Value(api.Attributes.PrivateKey)
	m.RemoteHostKey = types.StringValue(api.Attributes.RemoteHostKey)
	m.ConnectTimeout = types.Int64Value(api.Attributes.ConnectTimeout)

	return diags
}

// responseToDataSourceModel maps a keychainSSHConnectionAPI response onto a
// KeychainSSHConnectionDataSourceModel.
func responseToDataSourceModel(api *keychainSSHConnectionAPI, m *KeychainSSHConnectionDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if api.Attributes.Redacted {
		return redactedAttributesError()
	}

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Host = types.StringValue(api.Attributes.Host)
	m.Port = types.Int64Value(api.Attributes.Port)
	m.Username = types.StringValue(api.Attributes.Username)
	m.PrivateKeyID = types.Int64Value(api.Attributes.PrivateKey)
	m.RemoteHostKey = types.StringValue(api.Attributes.RemoteHostKey)
	m.ConnectTimeout = types.Int64Value(api.Attributes.ConnectTimeout)

	return diags
}
