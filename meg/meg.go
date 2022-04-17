package meg

import (
	"errors"
	"fmt"
	"github.com/t3rm1n4l/go-mega"
	"os"
	"strings"
)

func UploadToMega(m *mega.Mega, destNode *mega.Node, filePath string, filename string) error {
	_, err := m.UploadFile(filePath, destNode, filename, nil)
	if err != nil {
		return fmt.Errorf("uploading file failed: %w", err)
	}
	return nil
}

func ResolvePathOnMega(m *mega.Mega, dest string) ([]*mega.Node, error) {
	filePathList := strings.Split(dest, string(os.PathSeparator))
	root := m.FS.GetRoot()
	foundNodes, err := m.FS.PathLookup(root, filePathList)
	if err != nil {
		return nil, err
	}
	if len(foundNodes) == 0 {
		return nil, errors.New("path is empty for destination " + dest)
	}
	return foundNodes, nil
}

func CheckExists(m *mega.Mega, destNode *mega.Node, filename string) (exists bool, err error) {
	children, err := m.FS.GetChildren(destNode)
	if err != nil {
		return false, err
	}
	for _, childNode := range children {
		if childNode.GetName() == filename {
			return true, nil
		}
	}
	return false, nil
}
