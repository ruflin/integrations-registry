// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

var packageList = &Packages{}

type Packages struct {
	sync.RWMutex
	List map[string]*Package
}

func GetPackage(path string) (*Package, error) {
	p, err := packageList.getPackageFromCache(path)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}

	newPackage, err := NewPackage(path)
	if err != nil {
		return nil, err
	}

	packageList.addPackage(newPackage)

	return newPackage, nil
}

func (p *Packages) addPackage(p2 *Package) {
	p.Lock()
	defer p.Unlock()
	if p.List == nil {
		p.List = map[string]*Package{}
	}
	p.List[p2.GetPath()] = p2
}

func (p *Packages) getPackageFromCache(path string) (*Package, error) {
	p.RLock()
	defer p.RUnlock()
	// If cache exists, return it
	if _, ok := p.List[path]; ok {
		return p.List[path], nil
	}
	return nil, nil
}

// GetPackages returns a slice with all existing packages.
// The List is stored in memory and on the second request directly served from memory.
// This assumes changes to packages only happen on restart (unless development mode is enabled).
// Caching the packages request many file reads every time this method is called.
func GetPackages(packagesBasePaths []string) (*Packages, error) {
	if packageList != nil {
		return packageList, nil
	}

	var err error
	packageList, err = getPackagesFromFilesystem(packagesBasePaths)
	if err != nil {
		return nil, errors.Wrapf(err, "reading packages from filesystem failed")
	}
	return packageList, nil
}

func getPackagesFromFilesystem(packagesBasePaths []string) (*Packages, error) {
	packagePaths, err := getPackagePaths(packagesBasePaths)
	if err != nil {
		return nil, err
	}

	var pList = &Packages{}
	for _, path := range packagePaths {
		p, err := GetPackage(path)
		if err != nil {
			return nil, errors.Wrapf(err, "loading package failed (path: %s)", path)
		}

		pList.addPackage(p)
	}
	return pList, nil
}

// getPackagePaths returns List of available packages, one for each version.
func getPackagePaths(allPaths []string) ([]string, error) {
	var foundPaths []string
	for _, packagesPath := range allPaths {
		log.Printf("Packages in %s:", packagesPath)
		err := filepath.Walk(packagesPath, func(path string, info os.FileInfo, err error) error {
			relativePath, err := filepath.Rel(packagesPath, path)
			if err != nil {
				return err
			}

			dirs := strings.Split(relativePath, string(filepath.Separator))
			if len(dirs) < 2 {
				return nil // need to go to the package version level
			}

			if info.IsDir() {
				log.Printf("%-20s\t%10s\t%s", dirs[0], dirs[1], path)
				foundPaths = append(foundPaths, path)
			}
			return filepath.SkipDir // don't need to go deeper
		})
		if err != nil {
			return nil, errors.Wrapf(err, "listing packages failed (path: %s)", packagesPath)
		}
	}
	return foundPaths, nil
}
