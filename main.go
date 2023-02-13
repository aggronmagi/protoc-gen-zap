package main

import (
	"bytes"
	"strings"

	"github.com/aggronmagi/protoc-gen-zap/utils"
	"github.com/golang/protobuf/proto"
	plugin_go "github.com/golang/protobuf/protoc-gen-go/plugin"
	"github.com/pseudomuto/protokit"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"fmt"
	"log"
	_ "github.com/aggronmagi/protoc-gen-zap/codegen"
)

func main() {
	if err := protokit.RunPlugin(new(plugin)); err != nil {
		log.Fatal(err)
	}
}

type plugin struct{}

func (p *plugin) Generate(req *plugin_go.CodeGeneratorRequest) (*plugin_go.CodeGeneratorResponse, error) {
	descriptors := protokit.ParseCodeGenRequest(req)

	resp := new(plugin_go.CodeGeneratorResponse)

	for _, d := range descriptors {
		o := generateProtoZapFile(d)
		if o != nil {
			resp.File = append(resp.File, &plugin_go.CodeGeneratorResponse_File{
				Name:    &o.file,
				Content: proto.String(string(o.content)),
			})
		}
	}

	// buf := new(bytes.Buffer)
	// enc := json.NewEncoder(buf)
	// enc.SetIndent("", "  ")

	// if err := enc.Encode(files); err != nil {
	// 	return nil, err
	// }

	return resp, nil
}

type output struct {
	file    string
	content []byte
}

func generateProtoZapFile(fd *protokit.FileDescriptor) *output {
	buf := &bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("package %s", fd.GetPackage()))
	buf.WriteByte('\n')
	log.Println(fd.GetName(), "----------------------")
	for _, msg := range fd.Messages {
		//log.Println(utils.Sdump(msg, msg.GetName()))
		dumpMessage(msg, 0)
	}
	return &output{
		file:    strings.Replace(fd.GetName(), ".proto", ".zap.go", -1),
		content: buf.Bytes(),
	}
}

func dumpMessage(msg *protokit.Descriptor, n int) {
	log.Println(strings.Repeat("    ", n), msg.GetName(), msg.GetLongName(), msg.GetFullName(), msg.GetOptions().GetMapEntry())
	if msg.Options != nil && len(msg.Options.UninterpretedOption) > 0 {
		log.Println(" ", utils.Sdump(msg.Options.UninterpretedOption, "options"))
	}
	if msg.OptionExtensions != nil {
		log.Println("----------", len(msg.OptionExtensions), utils.Sdump(msg.OptionExtensions, "extern"))
	}
	//log.Println("############", msg.Extensions, msg.Extension, msg.Options, msg.OptionExtensions)
	if msg.Options != nil {
		log.Println("#####", msg.Options, msg.Options.UninterpretedOption, msg.OptionExtensions)
		msg.Options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			log.Println(fd, v)
			return true
		})
	}

	for _, field := range msg.Fields {
		if field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			log.Printf("%s >>> %s %s = %d", strings.Repeat("    ", n), field.GetTypeName(), field.GetName(), field.GetNumber())
		} else {
			log.Printf("%s >>> %s %s = %d", strings.Repeat("    ", n), field.GetType().String(), field.GetName(), field.GetNumber())
		}

		if field.Options != nil && len(field.Options.UninterpretedOption) > 0 {
			log.Println(" ----------------", utils.Sdump(field.Options.UninterpretedOption, "options"))
		}

	}
	for _, msg := range msg.Messages {
		dumpMessage(msg, n+1)
	}
}

type file struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Services    []*service `json:"services"`
}

func newFile(fd *protokit.FileDescriptor) *file {
	svcs := make([]*service, len(fd.GetServices()))
	for i, sd := range fd.GetServices() {
		svcs[i] = newService(sd)
	}

	return &file{
		Name:        fmt.Sprintf("%s.%s", fd.GetPackage(), fd.GetName()),
		Description: fd.GetComments().String(),
		Services:    svcs,
	}
}

type service struct {
	Name    string    `json:"name"`
	Methods []*method `json:"methods"`
}

func newService(sd *protokit.ServiceDescriptor) *service {
	methods := make([]*method, len(sd.GetMethods()))
	for i, md := range sd.GetMethods() {
		methods[i] = newMethod(md)
	}

	return &service{Name: sd.GetName(), Methods: methods}
}

type method struct {
	Name      string   `json:"name"`
	HTTPRules []string `json:"http_rules"`
}

func newMethod(md *protokit.MethodDescriptor) *method {
	httpRules := make([]string, 0)
	if httpRule, ok := md.OptionExtensions["google.api.http"].(*annotations.HttpRule); ok {
		switch httpRule.GetPattern().(type) {
		case *annotations.HttpRule_Get:
			httpRules = append(httpRules, fmt.Sprintf("GET %s", httpRule.GetGet()))
		case *annotations.HttpRule_Put:
			httpRules = append(httpRules, fmt.Sprintf("PUT %s", httpRule.GetPut()))
		case *annotations.HttpRule_Post:
			httpRules = append(httpRules, fmt.Sprintf("POST %s", httpRule.GetPost()))
		case *annotations.HttpRule_Delete:
			httpRules = append(httpRules, fmt.Sprintf("DELETE %s", httpRule.GetDelete()))
		case *annotations.HttpRule_Patch:
			httpRules = append(httpRules, fmt.Sprintf("PATCH %s", httpRule.GetPatch()))
		}
		// Append more for each rule in httpRule.AdditionalBindings...
	}

	return &method{Name: md.GetName(), HTTPRules: httpRules}
}
