package main

import (
	"encoding/json"
	"log/slog"

	"github.com/aws/jsii-runtime-go"
	"github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

func mutateAnnotations(objCdk8s cdk8s.ApiObject, objMeta *metav1.ObjectMeta) {
	defaultAnnotations := map[string]string{
		"sidecar.istio.io/inject": "true",
	}

	if objMeta.Annotations == nil {
		objMeta.Annotations = make(map[string]string)
	}

	for key, value := range defaultAnnotations {
		objMeta.Annotations[key] = value
	}

	objCdk8s.AddJsonPatch(cdk8s.JsonPatch_Replace(jsii.String("/metadata/annotations"), objMeta.Annotations))
}

func mutate(chart cdk8s.Helm) error {
	decoder := jsonserializer.NewSerializerWithOptions(
		jsonserializer.DefaultMetaFactory, // jsonserializer.MetaFactory
		scheme.Scheme,                     // runtime.Scheme implements runtime.ObjectCreater
		scheme.Scheme,                     // runtime.Scheme implements runtime.ObjectTyper
		jsonserializer.SerializerOptions{
			Yaml:   false,
			Pretty: false,
			Strict: false,
		},
	)

	for _, obj := range *chart.ApiObjects() {
		// Convert cdk8s object to bytes
		bytes, err := json.Marshal(obj.ToJson())
		if err != nil {
			return err
		}

		// Decode JSON to runtime object
		decoded, err := runtime.Decode(decoder, bytes)
		if err != nil {
			return err
		}
		kind := decoded.GetObjectKind().GroupVersionKind().Kind

		// Convert generic runtime object to specific typed object
		// This is the entrypoint for all the mutations
		switch kind {
		case "Deployment":
			deployment := decoded.(*appsv1.Deployment)
			mutateAnnotations(obj, &deployment.ObjectMeta)
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
