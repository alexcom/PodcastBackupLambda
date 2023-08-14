package meg

import (
	"errors"
	"fmt"
	"github.com/t3rm1n4l/go-mega"
	"os"
	"strings"
)

type MegaClient struct {
	mega *mega.Mega
	node *mega.Node
}

func NewMegaClient(email, password string) (*MegaClient, error) {
	if email == "" || password == "" {
		return nil, errors.New("username or password empty. set env MEGA_EMAIL & MEGA_PASSWORD")
	}
	m := mega.New()
	err := m.Login(email, password)
	if err != nil {
		return nil, fmt.Errorf("login failed : %w", err)
	}
	return &MegaClient{
		mega: m,
	}, nil
}

func (m *MegaClient) ChDir(dest string) error {
	fmt.Printf("changing location to %s", dest)
	filePathList := strings.Split(dest, string(os.PathSeparator))
	root := m.mega.FS.GetRoot()
	foundNodes, err := m.mega.FS.PathLookup(root, filePathList)
	if err != nil {
		return err
	}
	if len(foundNodes) == 0 {
		return errors.New("path is empty for destination " + dest)
	}
	m.node = foundNodes[len(foundNodes)-1]
	return nil
}

func (m *MegaClient) Upload(filePath string, filename string) error {
	if m.node == nil {
		return errors.New("cannot upload to unknown location. Use ChDir first")
	}
	_, err := m.mega.UploadFile(filePath, m.node, filename, nil)
	if err != nil {
		return fmt.Errorf("uploading file failed: %w", err)
	}
	return nil
}

func (m *MegaClient) Exists(filename string) (bool, error) {
	children, err := m.mega.FS.GetChildren(m.node)
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
