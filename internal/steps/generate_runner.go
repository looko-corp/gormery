package steps

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	config "github.com/myyrakle/gormery/internal/config"
)

func GenerateRunner(configFile config.ConfigFile, targets ProecssFileContexts) {
	// Runner-Path 디렉토리가 존재하지 않는다면 생성
	if _, err := os.Stat(configFile.RunnerPath); os.IsNotExist(err) {
		os.MkdirAll(configFile.RunnerPath, 0755)
	}

	var code string

	code += "package main\n\n"

	code += "import (\n"

	code += "\t " + `"sync"` + "\n"
	code += "\t " + `"os"` + "\n"
	code += "\t " + `"fmt"` + "\n"
	code += "\t " + `"strings"` + "\n\n"

	targetImport := `target "` + configFile.ModuleName + "/" + configFile.Basedir + `"`
	code += "\t" + targetImport + "\n"

	gormImport := `gormSchema "gorm.io/gorm/schema"`
	code += "\t" + gormImport + "\n"

	code += ")\n\n"

	code += "func main() {\n"

	code += "\t" + `code := ""` + "\n"

	// _gorm.go 파일 미리 생성하는 코드 추가
	filenames := targets.UniquedFileNames()
	for i, filename := range filenames {
		gormFilePath := strings.Replace(filename, ".go", "", 1) + configFile.OutputSuffix

		index := strconv.Itoa(i)
		code += "\t" + "gormFilePath" + index + " := " + `"` + gormFilePath + `"` + "\n"
		code += "\t" + "f" + index + ", err := os.Create(" + "gormFilePath" + index + ")" + "\n"
		code += "\t" + "if err != nil {" + "\n"
		code += "\t\t" + "panic(err)" + "\n"
		code += "\t" + "}" + "\n"
		code += "\t" + `code = ""` + "\n"
		code += "\t" + `code += "// Code generated by gormery. DO NOT EDIT.\n"` + "\n"
		code += "\t" + `code += "package " + "` + targets[0].packageName + `" + "\n\n"` + "\n"

		code += "\t" + "_, err = " + "f" + index + ".WriteString(code)" + "\n"
		code += "\t" + "if err != nil {" + "\n"
		code += "\t\t" + "panic(err)" + "\n"
		code += "\t" + "}" + "\n"

		code += "\t" + "f" + index + ".Close()" + "\n"
	}

	for i, target := range targets {
		code += generateCodeForTarget(i, target)
		code += "\n"
	}

	code += "}\n\n"

	// Gorm 파일 생성 코드 주입
	code += generateCreateGormFileFunction(configFile)

	// 파일 생성
	filePath := configFile.RunnerPath + "/main.go"
	file, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	_, err = file.WriteString(code)
	if err != nil {
		panic(err)
	}
}

func generateCodeForTarget(i int, target ProecssFileContext) string {
	id := fmt.Sprintf("target_%d", i)
	targetTypename := target.structName
	filename := target.filename
	structName := target.structName
	tableName := ""

	if target.entityParam != nil && len(*target.entityParam) > 0 {
		tableName = *target.entityParam
	}

	code := fmt.Sprintf(`
	%s, err := gormSchema.ParseWithSpecialTableName(
		&target.%s{},
		&sync.Map{},
		&gormSchema.NamingStrategy{},
		"",
	)

	if err == nil {
		createGormFile(%s, "%s", "%s", "%s")
	}
`, id, targetTypename, id, filename, structName, tableName)

	return code
}

func generateCreateGormFileFunction(configFile config.ConfigFile) string {
	code := ""

	code += `var basedir = ` + `"` + configFile.Basedir + `"` + "\n"
	code += `var outputSuffix = ` + `"` + configFile.OutputSuffix + `"` + "\n"

	code += `func createGormFile(schema *gormSchema.Schema, filename string, structName string, tableName string) {` + "\n"
	code += "\t" + `gormFilePath := strings.Replace(filename, ".go", "", 1) + outputSuffix` + "\n"

	code += "\t" + `code := ""` + "\n"

	// TableName 메서드 구현
	code += "\t" + `code += "func (t " + structName + ") TableName() string {\n"` + "\n"
	code += "\t" + `if tableName != "" {` + "\n"
	code += "\t" + `code += "\treturn \""+ tableName + "\"\n"` + "\n"
	code += "\t" + `} else {` + "\n"
	code += "\t" + `code += "\treturn \"" + schema.Table + "\"\n"` + "\n"
	code += "\t" + `}` + "\n"
	code += "\t" + `code += "}\n\n"` + "\n\n"

	// StructName 메서드 구현
	code += "\t" + `code += "func (t " + structName + ") StructName() string {\n"` + "\n"
	code += "\t" + `code += "\treturn \"" + structName + "\"\n"` + "\n"
	code += "\t" + `code += "}\n\n"` + "\n\n"

	// column 상수 목록 생성 (const ColumnName = "column_name")

	code += "\t" + `columnConstantNames := []string{}` + "\n"
	code += "\t" + `for _, field := range schema.Fields {` + "\n"
	code += "\t\t" + `columnConstantName := structName + "_" + field.Name` + "\n"
	code += "\t\t" + `columnConstantExpression := "const " + columnConstantName + " = " + "\"" + field.DBName + "\"" + "\n"` + "\n"
	code += "\t\t" + `columnConstantNames = append(columnConstantNames, "\t\t"+columnConstantName+",")` + "\n"
	code += "\t\t" + "code += columnConstantExpression" + "\n"
	code += "\t" + `}` + "\n\n"

	// Columns 메서드 구현
	code += "\t" + `code += "\nfunc (t " + structName + ") Columns() []string {\n"` + "\n"
	code += "\t" + `code += "\treturn []string{\n" + strings.Join(columnConstantNames, "\n") + "\n\t}\n"` + "\n"
	code += "\t" + `code += "}\n\n"` + "\n\n"

	// Slice 타입 구현
	if configFile.Features.Contains(config.FeatureSlice) {
		// named type 명명
		code += "\t" + `sliceTypeName := gormSchema.NamingStrategy{ NoLowerCase: true }.TableName(structName)` + "\n"

		// slice type명과 struct type명이 같다면 slice type명에 접미사로 'List'를 붙임
		code += "\t" + `if sliceTypeName == structName {` + "\n"
		code += "\t\t" + `sliceTypeName += "List"` + "\n"
		code += "\t" + `}` + "\n"

		// named type 추가
		code += "\t" + `code += "type " + sliceTypeName + " []" + structName + "\n\n"` + "\n"

		// Len 메서드 추가
		code += "\t" + `code += "func (t " + sliceTypeName + ") Len() int {\n"` + "\n"
		code += "\t" + `code += "\treturn len(t)\n"` + "\n"
		code += "\t" + `code += "}\n\n"` + "\n"

		// IsEmpty 메서드 추가
		code += "\t" + `code += "func (t " + sliceTypeName + ") IsEmpty() bool {\n"` + "\n"
		code += "\t" + `code += "\treturn len(t) == 0\n"` + "\n"
		code += "\t" + `code += "}\n\n"` + "\n"

		// First 메서드 추가
		code += "\t" + `code += "func (t " + sliceTypeName + ") First() " + structName + " {\n"` + "\n"
		code += "\t" + `code += "\tif t.IsEmpty() {\n"` + "\n"
		code += "\t" + `code += "\t\treturn " + structName + "{}\n"` + "\n"
		code += "\t" + `code += "\t}\n"` + "\n"
		code += "\t" + `code += "\treturn t[0]\n"` + "\n"
		code += "\t" + `code += "}\n\n"` + "\n"
	}

	code += "\t" + `f, err := os.OpenFile(gormFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)` + "\n"
	code += "\t" + `if err != nil {` + "\n"
	code += "\t\t" + `panic(err)` + "\n"
	code += "\t}\n"
	code += "\tdefer f.Close()\n"
	code += "\t" + `_, err = fmt.Fprintln(f, code)` + "\n"
	code += `}`

	return code
}
