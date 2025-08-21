package modules

package utility

import (
    "os"
    "path/filepath"
    "strings"
)

func FilingExists(root, clientID, filingName string) bool {
    clientDir := filepath.Join(root, clientID)
    _ = os.MkdirAll(clientDir, 0755) // ensure dir exists

    suffix := "+" + sanitizeName(filingName)
    entries, _ := os.ReadDir(clientDir)
    for _, e := range entries {
        if strings.HasSuffix(e.Name(), suffix) {
            return true
        }
    }
    return false
}

func sanitizeName(name string) string {
    return strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
}