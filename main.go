// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The protoc-gen-go binary is a protoc plugin to generate Go code for
// both proto2 and proto3 versions of the protocol buffer language.
//
// For more information about the usage of this plugin, see:
// https://protobuf.dev/reference/go/go-generated.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const genGoDocURL = "https://protobuf.dev/reference/go/go-generated"
const grpcDocURL = "https://grpc.io/docs/languages/go/quickstart/#regenerate-grpc-code"

var (
	Version = "0.0.1"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Fprintf(os.Stdout, "%v %v\n", filepath.Base(os.Args[0]), Version)
		os.Exit(0)
	}
	if len(os.Args) == 2 && os.Args[1] == "--help" {
		fmt.Fprintf(os.Stdout, "See "+genGoDocURL+" for usage information.\n")
		os.Exit(0)
	}

	var (
		flags   flag.FlagSet
		plugins = flags.String("plugins", "", "deprecated option")
	)
	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		if *plugins != "" {
			return errors.New("protoc-gen-go: plugins are not supported; use 'protoc --go-grpc_out=...' to generate gRPC\n\n" +
				"See " + grpcDocURL + " for more information.")
		}
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			genZap(gen, f)
		}
		//gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return nil
	})
}

func genZap(gen *protogen.Plugin, file *protogen.File) {
	filename := file.GeneratedFilenamePrefix + ".zap.go"
	//log.Println(filename)
	g := gen.NewGeneratedFile(filename, file.GoImportPath)
	genGeneratedHeader(gen, g, file)
	g.P("package ", file.GoPackageName)
	g.P()

	g.Import(protogen.GoImportPath("go.uber.org/zap/zapcore"))
	g.QualifiedGoIdent(protogen.GoIdent{GoName: "Abc", GoImportPath: "go.uber.org/zap/zapcore"})

	for _, m := range file.Messages {
		genZapMessage(g, m)
	}
}

func genZapMessage(g *protogen.GeneratedFile, m *protogen.Message) {
	// object marshal
	g.Annotate(m.GoIdent.GoName, m.Location)
	g.P("func (x *", m.GoIdent, ") MarshalLogObject(enc zapcore.ObjectEncoder) error {")
	for _, field := range m.Fields {
		if field.Desc.IsWeak() {
			continue
		}

		keyName := field.GoName
		fieldName := field.GoName
		if field.Desc.IsMap() {
			g.P(`enc.AddObject("`, keyName, `", zapcore.ObjectMarshalerFunc(func(oe zapcore.ObjectEncoder) error {`)
			g.P(`for k,v := range x.`, fieldName, "{")
			funcName, fieldMethod := getFieldFunc(field.Message.Fields[1])
			g.P(fmt.Sprintf(`enc.Add%s(%s, v%s)`, funcName, getFieldMapKey(g, field.Message.Fields[0]), fieldMethod))
			g.P("}")
			g.P("return nil")
			g.P("}))")
			continue
		}
		funcName, fieldMethod := getFieldFunc(field)
		switch {
		case field.Desc.IsList():
			g.P(fmt.Sprintf(`enc.AddArray("%s", zapcore.ArrayMarshalerFunc(func(ae zapcore.ArrayEncoder) error {`, keyName))
			g.P("for _,v := range x.", fieldName, "{")
			g.P(fmt.Sprintf("ae.Append%s(v%s)", funcName, fieldMethod))
			g.P("}")
			g.P("return nil")
			g.P("}))")
		default:
			g.P(fmt.Sprintf(`enc.Add%s("%s", x.%s%s)`, funcName, keyName, fieldName, fieldMethod))
		}
	}
	g.P("return nil")
	g.P("}")
	g.P()

	// array marshal
}

func getFieldMapKey(g *protogen.GeneratedFile, field *protogen.Field) (funcName string) {
	isImport := true
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		funcName = "strconv.FormatBool(k)"
	case protoreflect.EnumKind:
		funcName = "k.String()"
		isImport = false
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		funcName = "strconv.FormatInt64(int64(k), 10)"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		funcName = "strconv.FormatUint64(uint64(k), 10)"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		funcName = "strconv.FormatInt64(k, 10)"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		funcName = "strconv.FormatUint64(k, 10)"
	case protoreflect.FloatKind:
		funcName = "strconv.FormatFloat32(k, 10)"
	case protoreflect.DoubleKind:
		funcName = "strconv.FormatFloat64(k, 10)"
	case protoreflect.StringKind:
		funcName = "k"
		isImport = false
	case protoreflect.BytesKind:
		funcName = "{Invalid Map Key - []byte}"
		log.Printf("invalid map key type []byte. %#v", field)
	case protoreflect.MessageKind, protoreflect.GroupKind:
		funcName = "{Invalid Map Key - Object}"
		log.Printf("invalid map key type object. %#v", field)
	}
	if isImport {
		g.Import(protogen.GoImportPath("strconv"))
		g.QualifiedGoIdent(protogen.GoIdent{GoName: "Abc", GoImportPath: "strconv"})
	}
	return
}

func getFieldFunc(field *protogen.Field) (funcName, fieldMethod string) {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		funcName = "Bool"
	case protoreflect.EnumKind:
		funcName = "String"
		fieldMethod = ".String()"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		funcName = "Int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		funcName = "Uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		funcName = "Int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		funcName = "Uint64"
	case protoreflect.FloatKind:
		funcName = "Float32"
	case protoreflect.DoubleKind:
		funcName = "Float64"
	case protoreflect.StringKind:
		funcName = "String"
	case protoreflect.BytesKind:
		funcName = "Binary"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		funcName = "Object"
	}
	return
}

// fieldGoType returns the Go type used for a field.
//
// If it returns pointer=true, the struct field is a pointer to the type.
func fieldGoType(g *protogen.GeneratedFile, f *protogen.File, field *protogen.Field) (goType string, pointer bool) {
	if field.Desc.IsWeak() {
		return "struct{}", false
	}

	pointer = field.Desc.HasPresence()
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		goType = "bool"
	case protoreflect.EnumKind:
		goType = g.QualifiedGoIdent(field.Enum.GoIdent)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		goType = "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		goType = "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		goType = "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		goType = "uint64"
	case protoreflect.FloatKind:
		goType = "float32"
	case protoreflect.DoubleKind:
		goType = "float64"
	case protoreflect.StringKind:
		goType = "string"
	case protoreflect.BytesKind:
		goType = "[]byte"
		pointer = false // rely on nullability of slices for presence
	case protoreflect.MessageKind, protoreflect.GroupKind:
		goType = "*" + g.QualifiedGoIdent(field.Message.GoIdent)
		pointer = false // pointer captured as part of the type
	}
	switch {
	case field.Desc.IsList():
		return "[]" + goType, false
	case field.Desc.IsMap():
		keyType, _ := fieldGoType(g, f, field.Message.Fields[0])
		valType, _ := fieldGoType(g, f, field.Message.Fields[1])
		return fmt.Sprintf("map[%v]%v", keyType, valType), false
	}
	return goType, pointer
}

func genGeneratedHeader(gen *protogen.Plugin, g *protogen.GeneratedFile, f *protogen.File) {
	g.P("// Code generated by protoc-gen-zap. DO NOT EDIT.")

	g.P("// versions:")
	protocGenGoVersion := Version
	protocVersion := "(unknown)"
	if v := gen.Request.GetCompilerVersion(); v != nil {
		protocVersion = fmt.Sprintf("v%v.%v.%v", v.GetMajor(), v.GetMinor(), v.GetPatch())
		if s := v.GetSuffix(); s != "" {
			protocVersion += "-" + s
		}
	}
	g.P("// \tprotoc-gen-zap ", protocGenGoVersion)
	g.P("// \tprotoc        ", protocVersion)

	if f.Proto.GetOptions().GetDeprecated() {
		g.P("// ", f.Desc.Path(), " is a deprecated file.")
	} else {
		g.P("// source: ", f.Desc.Path())
	}
	g.P()
}
