package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-coolify/internal/provider/generated/resource_application_envs"
)

func testEnv(key string, preview bool) resource_application_envs.ApplicationEnvsModel {
	return resource_application_envs.ApplicationEnvsModel{
		Key:       types.StringValue(key),
		IsPreview: types.BoolValue(preview),
	}
}

func envAttribute(t *testing.T, name string) schema.Attribute {
	t.Helper()
	var resp resource.SchemaResponse
	NewApplicationEnvsResource().Schema(t.Context(), resource.SchemaRequest{}, &resp)

	block, ok := resp.Schema.Blocks["env"]
	if !ok {
		t.Fatal("no env block")
	}
	attr, ok := block.GetNestedObject().GetAttributes()[name].(schema.Attribute)
	if !ok {
		t.Fatalf("no env.%s attribute", name)
	}
	return attr
}

// The API returns creation order with preview envs interleaved; a configuration
// built from maps lists regular envs by key, then preview envs by key.
func TestSortEnvs(t *testing.T) {
	envs := []resource_application_envs.ApplicationEnvsModel{
		testEnv("TOKEN", false),
		testEnv("ORG", true),
		testEnv("CRON", false),
		testEnv("TOKEN", true),
		testEnv("ORG", false),
	}

	sortEnvs(envs)

	want := []struct {
		key     string
		preview bool
	}{
		{"CRON", false},
		{"ORG", false},
		{"TOKEN", false},
		{"ORG", true},
		{"TOKEN", true},
	}
	for i, w := range want {
		got := envs[i]
		if got.Key.ValueString() != w.key || got.IsPreview.ValueBool() != w.preview {
			t.Fatalf("position %d: got %s/preview=%t, want %s/preview=%t",
				i, got.Key.ValueString(), got.IsPreview.ValueBool(), w.key, w.preview)
		}
	}
}

func TestApplicationEnvValueIsSensitive(t *testing.T) {
	if !envAttribute(t, "value").IsSensitive() {
		t.Fatal("env.value must be sensitive")
	}
}

func TestApplicationEnvIsBuildTimeIsReadOnly(t *testing.T) {
	attr := envAttribute(t, "is_build_time")
	if attr.IsOptional() || attr.IsRequired() || !attr.IsComputed() {
		t.Fatal("env.is_build_time must be read-only")
	}
}

// Unknown is what an unset Optional+Computed flag looks like in a plan.
func TestEnvWriteBodiesOmitIsBuildTime(t *testing.T) {
	env := testEnv("KEY", false)
	env.Value = types.StringValue("v")
	env.IsBuildTime = types.BoolUnknown()

	for name, body := range map[string]any{
		"create": createEnvBody(env),
		"bulk":   bulkEnvItem(env),
	} {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "is_build_time") {
			t.Errorf("%s body sends is_build_time: %s", name, raw)
		}
	}
}
