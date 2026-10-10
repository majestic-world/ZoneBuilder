package main

import (
	"errors"
	"log"
	"strings"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/spawn"
	"zonebuilder/internal/zonexml"
)

// compile compiles every area into one spawn file named name (fallback when
// name is blank) for the XML window. Nothing comes out while the
// compilation is blocked; the status then names the first blocking problem.
func (e *spawnEditor) compile(name, fallback string) (locale.Message, []zonexml.File) {
	if strings.TrimSpace(name) == "" {
		name = fallback
	}
	areas := e.doc.Areas()
	switch {
	case e.drawing:
		return locale.Message{Key: "spawn.compile.drawing"}, nil
	case len(areas) == 0:
		return locale.Message{Key: "spawn.compile.empty"}, nil
	}
	f, err := e.doc.Compile(name)
	if b, ok := errors.AsType[*spawn.BlockedError](err); ok {
		log.Printf("spawn: %v", b)
		return locale.Message{Key: "spawn.compile.blocked", Count: len(b.Problems), Plural: true, Parts: map[string]locale.Message{"problem": b.Problems[0].Message()}}, nil
	}
	if err != nil {
		log.Printf("spawn: compilação: %v", err)
		return locale.Message{Key: "spawn.compile.failed", Args: map[string]string{"detail": err.Error()}}, nil
	}
	points := 0
	for _, a := range areas {
		points += len(a.Points)
	}
	log.Printf("spawn: compilado %s: %s de %s", f.Name, inflect.Count(points, "ponto", "pontos"), inflect.Count(len(areas), "área", "áreas"))
	return locale.Message{
		Key: "spawn.compile.done", Count: points, Plural: true,
		Args:  map[string]string{"file": f.Name},
		Parts: map[string]locale.Message{"areas": {Key: "spawn.areas.count", Count: len(areas), Plural: true}},
	}, []zonexml.File{zonexml.File(f)}
}
