// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Gendex generates a dex file used by Go apps created with gomobile.
//
// The dex is a thin extension of NativeActivity, providing access to
// a few platform features (not the SDK UI) not easily accessible from
// NDK headers. Long term these could be made part of the standard NDK,
// however that would limit gomobile to working with newer versions of
// the Android OS, so we do this while we wait.
//
// Respects ANDROID_HOME to set the path of the Android SDK.
// javac must be on the PATH.
package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nandoxscr/mobile/internal/sdkpath"
)

var outfile = flag.String("o", "", "result will be written file")

var tmpdir string

func main() {
	flag.Parse()

	var err error
	tmpdir, err = ioutil.TempDir("", "gendex-")
	if err != nil {
		log.Fatal(err)
	}

	err = gendex()
	os.RemoveAll(tmpdir)
	if err != nil {
		log.Fatal(err)
	}
}

func gendex() error {
	androidHome, err := sdkpath.AndroidHome()
	if err != nil {
		return fmt.Errorf("couldn't find Android SDK: %w", err)
	}
	if err := os.MkdirAll(tmpdir+"/work/org/golang/app", 0775); err != nil {
		return err
	}
	javaFiles, err := filepath.Glob("../../app/*.java")
	if err != nil {
		return err
	}
	if len(javaFiles) == 0 {
		return errors.New("could not find ../../app/*.java files")
	}
	platform, err := findLast(androidHome + "/platforms")
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"javac",
		"-source", "1.8",
		"-target", "1.8",
		"-bootclasspath", platform+"/android.jar",
		"-d", tmpdir+"/work",
	)
	cmd.Args = append(cmd.Args, javaFiles...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(cmd.Args)
		os.Stderr.Write(out)
		return err
	}
	buildTools, err := findLast(androidHome + "/build-tools")
	if err != nil {
		return err
	}
	cmd = exec.Command(
		buildTools+"/dx",
		"--dex",
		"--output="+tmpdir+"/classes.dex",
		tmpdir+"/work",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return err
	}
	src, err := ioutil.ReadFile(tmpdir + "/classes.dex")
	if err != nil {
		return err
	}
	data := base64.StdEncoding.EncodeToString(src)

	buf := new(bytes.Buffer)
	fmt.Fprint(buf, header)

	var piece string
	for len(data) > 0 {
		l := 70
		if l > len(data) {
			l = len(data)
		}
		piece, data = data[:l], data[l:]
		fmt.Fprintf(buf, "\t`%s` + \n", piece)
	}
	fmt.Fprintf(buf, "\t``")
	out, err := format.Source(buf.Bytes())
	if err != nil {
		buf.WriteTo(os.Stderr)
		return err
	}

	w, err := os.Create(*outfile)
	if err != nil {
		return err
	}
	if _, err := w.Write(out); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func findLast(path string) (string, error) {
	dir, err := os.Open(path)
	if err != nil {
		return "", err
	}
	children, err := dir.Readdirnames(-1)
	if err != nil {
		return "", err
	}
	return path + "/" + children[len(children)-1], nil
}

var header = `// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by gendex.go. DO NOT EDIT.

package main

var dexStr = `
