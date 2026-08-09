// Package scale plants a large tree, so the claims §2.3 and §24 make about cost
// can be measured rather than argued.
//
// It is test infrastructure that lives outside a _test.go file for one reason:
// the tree it plants is truth alone — state.toml files and nothing else — and
// building it is a few hundred lines of shape that the scale test, a benchmark,
// and anything later that wants a big tree all need. Planting truth rather than
// running `para add` N thousand times is deliberate and is also the faster
// thing: §2.4 lists "a merge resolved truth and left the projections wrong"
// among the states `rebuild` exists for, so a planted tree plus one `rebuild` is
// a tree para itself produced.
package scale

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Shape is how many of each kind a planted tree holds.
//
// The proportions are meant to look like a tree somebody uses: a lot of
// projects, each with a couple of objectives, each with a couple of key-results,
// and a long tail of areas and resources. What matters for the measurements is
// the total and the depth, not the exact mix.
type Shape struct {
	Projects            int
	ObjectivesPerProj   int
	KeyResultsPerObj    int
	Areas               int
	SubAreasPerArea     int
	Resources           int
	Skills              int
	ArchivedPerBucket   int
	MeasurementsPerKR   int
	NotesPerProject     int
	ProjectsWithJournal int
}

// Default is the shape the scale test uses: a few thousand entities.
//
//	200 projects
//	× 2 objectives = 400
//	× 2 key-results = 800
//	+ 300 areas + 300 sub-areas + 400 resources + 20 skills + 60 archived
//	≈ 2500 entities, plus a container per project and per objective.
func Default() Shape {
	return Shape{
		Projects:            200,
		ObjectivesPerProj:   2,
		KeyResultsPerObj:    2,
		Areas:               300,
		SubAreasPerArea:     1,
		Resources:           400,
		Skills:              20,
		ArchivedPerBucket:   20,
		MeasurementsPerKR:   3,
		NotesPerProject:     4,
		ProjectsWithJournal: 50,
	}
}

// Entities is how many addressable entities the shape describes, which is the
// number the scale claim is about. Containers are not counted: they are
// structure rather than things somebody filed.
func (s Shape) Entities() int {
	objectives := s.Projects * s.ObjectivesPerProj
	return s.Projects +
		objectives +
		objectives*s.KeyResultsPerObj +
		s.Areas + s.Areas*s.SubAreasPerArea +
		s.Resources + s.Skills + 3*s.ArchivedPerBucket
}

// Plant writes a tree of the given shape at root: tree.toml, the seven buckets,
// and every entity's .para/state.toml and journal. It writes no projection —
// that is `rebuild`'s job, and running it is part of what the scale test
// measures.
func Plant(root string, s Shape) error {
	files := map[string]string{
		".para/tree.toml": "schema = 1\nname = \"brain\"\ndescription = \"A large tree.\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
	}
	for _, bucket := range []string{
		"projects", "areas", "resources", "archive",
		"archive/projects", "archive/areas", "archive/resources",
	} {
		files[bucket+"/.para/state.toml"] = state(title(bucket), "A bucket.")
	}

	for i := 0; i < s.Projects; i++ {
		project := fmt.Sprintf("projects/p%04d", i)
		files[project+"/.para/state.toml"] = stateWith(
			fmt.Sprintf("Project %d", i), "A project.", "status = \"in-progress\"\n"+tagLine(i))
		if i < s.ProjectsWithJournal {
			files[project+"/.para/logs/20260101T000000Z.jsonl"] = notes(s.NotesPerProject, i)
		}

		files[project+"/objectives/.para/state.toml"] = state("Objectives", "What this project is trying to move.")
		for j := 0; j < s.ObjectivesPerProj; j++ {
			objective := fmt.Sprintf("%s/objectives/o%d", project, j)
			files[objective+"/.para/state.toml"] = stateWith(
				fmt.Sprintf("Objective %d.%d", i, j), "An objective.", "status = \"in-progress\"\n")
			files[objective+"/key-results/.para/state.toml"] = state("Key results", "How this objective is measured.")
			for k := 0; k < s.KeyResultsPerObj; k++ {
				kr := fmt.Sprintf("%s/key-results/k%d", objective, k)
				files[kr+"/.para/state.toml"] = stateWith(
					fmt.Sprintf("Key result %d.%d.%d", i, j, k), "A key result.",
					"type = \"ratio\"\nstart = \"100/1000\"\ntarget = \"900/1000\"\n")
				files[kr+"/.para/logs/20260101T000000Z.jsonl"] = measurements(s.MeasurementsPerKR)
			}
		}
	}

	for i := 0; i < s.Areas; i++ {
		area := fmt.Sprintf("areas/a%04d", i)
		files[area+"/.para/state.toml"] = state(fmt.Sprintf("Area %d", i), "An area.")
		for j := 0; j < s.SubAreasPerArea; j++ {
			files[fmt.Sprintf("%s/s%d/.para/state.toml", area, j)] = state(
				fmt.Sprintf("Sub-area %d.%d", i, j), "A nested area.")
		}
	}
	for i := 0; i < s.Resources; i++ {
		files[fmt.Sprintf("resources/r%04d/.para/state.toml", i)] = state(
			fmt.Sprintf("Resource %d", i), "A resource.")
	}
	for i := 0; i < s.Skills; i++ {
		files[fmt.Sprintf(".agents/skills/para-sk%03d/.para/state.toml", i)] = state(
			fmt.Sprintf("Skill %d", i), fmt.Sprintf("when asked about %d", i))
	}
	for _, bucket := range []string{"projects", "areas", "resources"} {
		for i := 0; i < s.ArchivedPerBucket; i++ {
			files[fmt.Sprintf("archive/%s/old%03d/.para/state.toml", bucket, i)] = state(
				fmt.Sprintf("Archived %s %d", bucket, i), "Something that has left the live tree.")
		}
	}

	return writeAll(root, files)
}

func state(name, description string) string {
	return stateWith(name, description, "")
}

func stateWith(name, description, extra string) string {
	return fmt.Sprintf("name = %q\ndescription = %q\n%screated = \"2026-01-01T00:00:00Z\"\n", name, description, extra)
}

// tagLine gives a third of the projects a tag, so `--tags` has something to
// filter and something to reject.
func tagLine(i int) string {
	if i%3 != 0 {
		return ""
	}
	return "tags = [\"kafka\", \"consumer\"]\n"
}

func notes(n, seed int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "{\"at\":\"2026-01-%02dT09:00:00Z\",\"kind\":\"note\",\"note\":\"note %d on project %d\"}\n",
			1+i%28, i, seed)
	}
	return b.String()
}

func measurements(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "{\"at\":\"2026-01-%02dT09:00:00Z\",\"kind\":\"measurement\",\"value\":\"%d/1000\"}\n",
			1+i%28, 100+i*50)
	}
	return b.String()
}

func title(bucket string) string {
	parts := strings.Split(bucket, "/")
	last := parts[len(parts)-1]
	return strings.ToUpper(last[:1]) + last[1:]
}

func writeAll(root string, files map[string]string) error {
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	// Every .para/ has the same shape (§5.1, §8.4), so a logs/ directory exists
	// even where no event has been written into it.
	for rel := range files {
		if !strings.HasSuffix(rel, "/.para/state.toml") && rel != ".para/tree.toml" {
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/state.toml")), "logs")
		if rel == ".para/tree.toml" {
			dir = filepath.Join(root, ".para", "logs")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.MkdirAll(filepath.Join(root, ".agents", "rules"), 0o755)
}
