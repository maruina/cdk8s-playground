package main

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/jsii-runtime-go"
	"github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getContainers(u *unstructured.Unstructured) []interface{} {
	containers, found, err := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "containers")
	if !found || err != nil {
		return nil
	}
	return containers
}

func mutateContainersEnv(object cdk8s.ApiObject, u *unstructured.Unstructured) {
	containers := getContainers(u)
	if containers == nil {
		return
	}

	for i, container := range containers {
		c := container.(map[string]interface{})
		env, found, err := unstructured.NestedSlice(c, "env")
		if !found || err != nil {
			continue
		}

		env = append(env, map[string]interface{}{
			"name":  "NEW_ENV",
			"value": "new-value",
		})

		object.AddJsonPatch(cdk8s.JsonPatch_Replace(jsii.String(fmt.Sprintf("/spec/template/spec/containers/%d/env", i)), env))
	}
}

func mutateAnnotations(objCdk8s cdk8s.ApiObject, u *unstructured.Unstructured) {
	defaultAnnotation := map[string]string{
		"sidecar.istio.io/inject": "unstructured",
	}

	existing := u.GetAnnotations()
	if existing == nil {
		existing = make(map[string]string)
	}
	for k, v := range defaultAnnotation {
		existing[k] = v
	}

	objCdk8s.AddJsonPatch(cdk8s.JsonPatch_Replace(jsii.String("/metadata/annotations"), existing))
}

func mutate(chart cdk8s.Helm) error {
	for _, obj := range *chart.ApiObjects() {
		u := unstructured.Unstructured{}
		// Convert cdk8s object to bytes
		bytes, err := json.Marshal(obj.ToJson())
		if err != nil {
			return err
		}

		// Unmarshal bytes to unstructured object
		err = u.UnmarshalJSON(bytes)
		if err != nil {
			return err
		}
		kind := u.GroupVersionKind().Kind

		// This is the entrypoint for all the mutations
		switch kind {
		case "Deployment":
			mutateAnnotations(obj, &u)
			mutateContainersEnv(obj, &u)
		default:
			// Do nothing
			slog.Info("Skipping object", "kind", kind)
		}
	}
	return nil
}

func main() {
	hid := "podinfo"

	app := cdk8s.NewApp(&cdk8s.AppProps{
		Outdir:         jsii.String("dist"),
		YamlOutputType: cdk8s.YamlOutputType_FOLDER_PER_CHART_FILE_PER_RESOURCE,
	})

	c := cdk8s.NewChart(app, jsii.String("test-chart"), &cdk8s.ChartProps{})
	h := cdk8s.NewHelm(c, jsii.String(hid), &cdk8s.HelmProps{
		Chart:       jsii.String("oci://ghcr.io/stefanprodan/charts/podinfo"),
		Version:     jsii.String("6.6.2"),
		ReleaseName: jsii.String(hid),
		HelmFlags:   jsii.Strings("--no-hooks", "--namespace", hid),
	})

	if err := mutate(h); err != nil {
		panic(err)
	}

	app.Synth()
}
