// DBDeployer - The MySQL Sandbox
// Copyright © 2006-2018 Giuseppe Maxia
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	//"bufio"
	"fmt"
	"github.com/datacharmer/dbdeployer/common"
	"os"
	"strings"
	"time"
)

func get_file_date(filename string) string {
	file, err := os.Stat(filename)
	if err != nil {
		common.Exit(1, fmt.Sprintf("Error getting timestamp for file %s (%s)", filename, err))
	}
	modified_time := file.ModTime()
	return modified_time.Format("2006-01-02")
}

func main() {
	templates_dir := ".build"
	if !common.DirExists(templates_dir) {
		os.Chdir("..")
		if !common.DirExists(templates_dir) {
			common.Exit(1, "Directory .build/ not found")
		}
	}
	version_dest_file := "common/version.go"

	if !common.FileExists(version_dest_file) {
		common.Exit(1, fmt.Sprintf("File %s not found", version_dest_file))
	}
	version_template := templates_dir + "/version_template.txt"
	template := common.SlurpAsString(version_template)

	version_file := templates_dir + "/VERSION"
	version := strings.TrimSpace(common.SlurpAsString(version_file))
	version_date := get_file_date(version_file)

	compatible_version_file := templates_dir + "/COMPATIBLE_VERSION"
	compatible_version := strings.TrimSpace(common.SlurpAsString(compatible_version_file))
	compatible_version_date := get_file_date(compatible_version_file)

	var data = common.StringMap{
		"Version":               version,
		"VersionDate":           version_date,
		"CompatibleVersion":     compatible_version,
		"CompatibleVersionDate": compatible_version_date,
		"Timestamp":             time.Now().Format("2006-01-02 15:04"),
	}
	version_code := common.TemplateFill(template, data)
	/*
		file, err := os.Open(version_dest_file)
		if err != nil {
			common.Exit(1, fmt.Sprintf("error opening file %s", version_dest_file))
		}
		defer file.Close()
		writer := bufio.NewWriter(file)
		written, err := writer.WriteString(version_code)
		if err != nil {
			common.Exit(1, fmt.Sprintf("error writing to file %s", version_dest_file))
		}
	*/
	common.WriteString(version_code, version_dest_file)
	// fmt.Printf("Written %d bytes into %s\n", written, version_dest_file)
}
