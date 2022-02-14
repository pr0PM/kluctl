package uo

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (uo *UnstructuredObject) GetK8sGVK() schema.GroupVersionKind {
	kind, _, err := uo.GetNestedString("kind")
	if err != nil {
		log.Fatal(err)
	}
	apiVersion, _, err := uo.GetNestedString("apiVersion")
	if err != nil {
		log.Fatal(err)
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		log.Fatal(err)
	}
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
}

func (uo *UnstructuredObject) SetK8sGVK(gvk schema.GroupVersionKind) {
	err := uo.SetNestedField(gvk.GroupVersion().String(), "apiVersion")
	if err != nil {
		log.Fatal(err)
	}
	err = uo.SetNestedField(gvk.Kind, "kind")
	if err != nil {
		log.Fatal(err)
	}
}

func (uo *UnstructuredObject) GetK8sName() string {
	s, _, err := uo.GetNestedString("metadata", "name")
	if err != nil {
		log.Fatal(err)
	}
	return s
}

func (uo *UnstructuredObject) SetK8sName(name string) {
	err := uo.SetNestedField(name, "metadata", "name")
	if err != nil {
		log.Fatal(err)
	}
}

func (uo *UnstructuredObject) GetK8sNamespace() string {
	s, _, err := uo.GetNestedString("metadata", "namespace")
	if err != nil {
		log.Fatal(err)
	}
	return s
}

func (uo *UnstructuredObject) SetK8sNamespace(name string) {
	err := uo.SetNestedField(name, "metadata", "namespace")
	if err != nil {
		log.Fatal(err)
	}
}

func (uo *UnstructuredObject) GetK8sLabels() map[string]string {
	ret, ok, err := uo.GetNestedStringMapCopy("metadata", "labels")
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		return map[string]string{}
	}
	return ret
}

func (uo *UnstructuredObject) GetK8sLabel(name string) *string {
	ret, ok, err := uo.GetNestedString("metadata", "labels", name)
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		return nil
	}
	return &ret
}

func (uo *UnstructuredObject) SetK8sLabel(name string, value string) {
	err := uo.SetNestedField(value, "metadata", "labels", name)
	if err != nil {
		log.Fatal(err)
	}
}

func (uo *UnstructuredObject) GetK8sAnnotations() map[string]string {
	ret, ok, err := uo.GetNestedStringMapCopy("metadata", "annotations")
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		return map[string]string{}
	}
	return ret
}

func (uo *UnstructuredObject) GetK8sAnnotation(name string) *string {
	ret, ok, err := uo.GetNestedString("metadata", "annotations", name)
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		return nil
	}
	return &ret
}

func (uo *UnstructuredObject) SetK8sAnnotation(name string, value string) {
	err := uo.SetNestedField(value, "metadata", "annotations", name)
	if err != nil {
		log.Fatal(err)
	}
}
