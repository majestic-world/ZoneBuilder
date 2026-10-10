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

// compilationProblem checks whether the editor is ready to request NPC IDs.
func (e *spawnEditor) compilationProblem(areaCount int) locale.Message {
	switch {
	case e.drawing:
		return locale.Message{Key: "spawn.compile.drawing"}
	case areaCount == 0:
		return locale.Message{Key: "spawn.compile.empty"}
	}
	return locale.Message{}
}

// compile writes every area with the IDs chosen for this compilation.
// Nothing comes out while compilation is blocked.
func (e *spawnEditor) compile(name, fallback string, npcIDs []int) (locale.Message, []zonexml.File) {
	if strings.TrimSpace(name) == "" {
		name = fallback
	}
	areas := e.doc.Areas()
	if msg := e.compilationProblem(len(areas)); msg.Key != "" {
		return msg, nil
	}
	f, err := e.doc.Compile(name, npcIDs)
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
