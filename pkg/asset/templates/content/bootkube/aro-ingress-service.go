package bootkube

import (
	"os"
	"path/filepath"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/templates/content"
)

const (
	aroIngressNamespaceFileName = "aro-ingress-namespace.yaml"
	aroIngressServiceFileName   = "aro-ingress-service.yaml.template"
)

var _ asset.WritableAsset = (*AROIngressService)(nil)

// AROIngressService is an asset for the openshift-apiserver namespace
type AROIngressService struct {
	FileList []*asset.File
}

// Dependencies returns all of the dependencies directly needed by the asset
func (t *AROIngressService) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Name returns the human-friendly name of the asset.
func (t *AROIngressService) Name() string {
	return "AROIngressService"
}

// Generate generates the actual files by this asset
func (t *AROIngressService) Generate(parents asset.Parents) error {
	t.FileList = nil

	for _, filename := range []string{aroIngressNamespaceFileName, aroIngressServiceFileName} {
		data, err := content.GetBootkubeTemplate(filename)
		if err != nil {
			return err
		}

		t.FileList = append(t.FileList, &asset.File{
			Filename: filepath.Join(content.TemplateDir, filename),
			Data:     []byte(data),
		})
	}

	return nil
}

// Files returns the files generated by the asset.
func (t *AROIngressService) Files() []*asset.File {
	return t.FileList
}

// Load returns the asset from disk.
func (t *AROIngressService) Load(f asset.FileFetcher) (bool, error) {
	ingressNamespaceData, err := f.FetchByName(filepath.Join(content.TemplateDir, aroIngressNamespaceFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	ingressServiceData, err := f.FetchByName(filepath.Join(content.TemplateDir, aroIngressServiceFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	t.FileList = []*asset.File{ingressNamespaceData, ingressServiceData}
	return true, nil
}
