package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"

	//"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func checkBazeliskExists() {
	path, err := exec.LookPath("bazelisk")
	if err != nil {
		fmt.Printf("Cannot find 'bazelisk' executable\n")
	} else {
		fmt.Printf("Found the 'bazelisk' executable is at '%s'\n", path)
	}
}

type MavenArtifactRequest2 struct {
	MavenArtifactCoordinates []string `json:"artifacts" binding:"required"`
}

func main() {
	router := gin.Default()

	router.POST("/pin", func(context *gin.Context) {
		var mavenArtifactRequest MavenArtifactRequest2
		errBindJson := context.BindJSON(&mavenArtifactRequest)
		if errBindJson != nil {
			fmt.Printf("Unable to parse JSON request: %v", errBindJson)
		}

		// Create a temp dir.
		tempDir, errTempDir := ioutil.TempDir("", "maven-install")
		if errTempDir != nil {
			fmt.Printf("Unable to create temp dir: %v", errTempDir)
		}
		//defer os.RemoveAll(tempDir)
		fmt.Println("Created a temp dir:", tempDir)

		// Create a new $tempDir/WORKSPACE file.
		filePathWorkspace := filepath.Join(tempDir, "WORKSPACE")
		fileContentWorkspace :=
			`load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

RULES_JVM_EXTERNAL_TAG = "3.3"
RULES_JVM_EXTERNAL_SHA = "d85951a92c0908c80bd8551002d66cb23c3434409c814179c0ff026b53544dab"
http_archive(
    name = "rules_jvm_external",
    url = "https://github.com/bazelbuild/rules_jvm_external/archive/%s.zip" % RULES_JVM_EXTERNAL_TAG,
    sha256 = RULES_JVM_EXTERNAL_SHA,
    strip_prefix = "rules_jvm_external-%s" % RULES_JVM_EXTERNAL_TAG,
)

load("@rules_jvm_external//:defs.bzl", "maven_install")
load("//:maven_artifacts.bzl", "maven_artifact_list")
maven_install(
    name = "maven",
    repositories = [
      "https://jcenter.bintray.com/",
      "https://maven.google.com",
      "https://repo1.maven.org/maven2",
    ],
    artifacts = maven_artifact_list(),
    strict_visibility = True,
    version_conflict_policy = "pinned",
)`
		if err := ioutil.WriteFile(filePathWorkspace, []byte(fileContentWorkspace), 0644); err != nil {
			fmt.Printf("Unable to write file: %v", err)
		}

		// Create a new $tempDir/BUILD.bazel file.
		filePathBuildBazel := filepath.Join(tempDir, "BUILD.bazel")
		fileContentBuildBazel :=
			`exports_files(
    ["maven_artifacts.bzl"],
)`
		if err := ioutil.WriteFile(filePathBuildBazel, []byte(fileContentBuildBazel), 0644); err != nil {
			fmt.Printf("Unable to write file: %v", err)
		}

		// Create a new $tempDir/maven_artifacts.bzl file.
		filePathMavenartifacts := filepath.Join(tempDir, "maven_artifacts.bzl")
		fileContentMavenartifacts :=
			`def maven_artifact_list():
  return`
		coordinates := []string{} // make an empty slice.
		for i, mavenArtifactCoordinate := range mavenArtifactRequest.MavenArtifactCoordinates {
			fmt.Println(i, mavenArtifactCoordinate)
			coordinates = append(coordinates, "\""+mavenArtifactCoordinate+"\"")
		}
		fileContentMavenartifacts = fileContentMavenartifacts + " [\n    " + strings.Join(coordinates, ",\n    ") + ",\n  ]"
		if err := ioutil.WriteFile(filePathMavenartifacts, []byte(fileContentMavenartifacts), 0644); err != nil {
			fmt.Printf("Unable to write file: %v", err)
		}

		// Execute the bazel command to generate a maven_install.json file.
		checkBazeliskExists()
		cmd := exec.Command("bazelisk", "run", "@maven//:pin")
		cmd.Dir = tempDir
		cmd.Run()

		// Read the maven_install.json file content into a string.
		filePathMavenInstallJson := filepath.Join(tempDir, "maven_install.json")
		fileContentMavenInstallJsonBytes, errReadMavenInstallJson := ioutil.ReadFile(filePathMavenInstallJson)
		if errReadMavenInstallJson != nil {
			fmt.Print("Unable to read file: %v", errReadMavenInstallJson)
		}
		fileContentMavenInstallJsonString := string(fileContentMavenInstallJsonBytes)

		// Produce the HTTP response.
		context.JSON(http.StatusOK, fileContentMavenInstallJsonString)
	})

	// Run the HTTP server.
	router.Run()
}
