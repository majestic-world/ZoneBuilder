// Command zbmodel extracts a skinned model from a Lineage II client into a
// UE2HUM01 bundle (internal/model), offline: the app embeds the result and
// never reads the client for it at runtime.
//
//	zbmodel -client <pasta> -mesh Pacote.Mesh -anim Pacote.Anim \
//	        -skins Pacote.Grupo.Skin0,Pacote.Grupo.Skin1 -clips Wait \
//	        [-drawscale 1] -out monster.bin
//
// It prints the collision radius (npcgrp.rs: the median horizontal distance
// of the vertices) and the height of the first clip's pose, both at the
// drawscale, which become the embedded model's constants.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/model"
)

func main() {
	fs := flag.NewFlagSet("zbmodel", flag.ExitOnError)
	client := fs.String("client", "", "pasta do cliente do Lineage II")
	mesh := fs.String("mesh", "", "SkeletalMesh, Pacote.Objeto")
	anim := fs.String("anim", "", "MeshAnimation, Pacote.Objeto")
	skins := fs.String("skins", "", "materiais separados por vírgula: 1 para todas as seções ou 1 por seção, na ordem dos slots")
	clips := fs.String("clips", "", "sequências separadas por vírgula; a primeira dá a pose dos pés e da altura")
	drawScale := fs.Float64("drawscale", 1, "escala do modelo")
	out := fs.String("out", "", "arquivo UE2HUM01 de saída")
	fs.Parse(os.Args[1:])
	if *client == "" || *mesh == "" || *anim == "" || *skins == "" || *clips == "" || *out == "" || fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "uso: zbmodel -client <pasta> -mesh <Pacote.Mesh> -anim <Pacote.Anim> -skins <skin,...> -clips <clip,...> [-drawscale 1] -out <arquivo>")
		os.Exit(2)
	}
	req := request{
		Mesh:      *mesh,
		Animation: *anim,
		Skins:     strings.Split(*skins, ","),
		Clips:     strings.Split(*clips, ","),
		DrawScale: float32(*drawScale),
	}
	res, err := extract(l2pkg.NewClient(*client), req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	data := model.Encode(res.Bundle)
	if _, err := model.Decode(data); err != nil {
		fmt.Fprintf(os.Stderr, "o bundle gerado não decodifica: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %s, %s, %s\n", req.Mesh,
		inflect.Count(res.Vertices, "vértice", "vértices"),
		inflect.Count(res.Bones, "osso", "ossos"),
		inflect.Count(len(res.Bundle.Parts[0].Sections), "seção", "seções"))
	for i, s := range res.Bundle.Parts[0].Sections {
		fmt.Printf("seção %d: %s\n", i, s.Texture)
	}
	fmt.Printf("raio %s\n", strconv.FormatFloat(float64(res.Radius), 'g', -1, 32))
	fmt.Printf("altura %s\n", strconv.FormatFloat(float64(res.Height), 'g', -1, 32))
	fmt.Printf("%s gravado (%s)\n", *out, inflect.Count(len(data), "byte", "bytes"))
}
