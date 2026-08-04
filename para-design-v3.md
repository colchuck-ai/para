# structure

.para-config.toml
.para/
  audit.log (jsonl)
  data.json
.agents/
  rules/
    para-rule-1.md
    ...
  skills/
    para-skill-1/
      SKILL.md
      ...
    ...
README.md
AGENTS.md
CLAUDE.md
ACTIVITY.md
projects/
  README.md
  AGENTS.md
  CLAUDE.md
  ACTIVITY.md
  <project-id>/
    README.md
    ACTIVITY.md
    objectives/
      <objective-id>/
        README.md
        ACTIVITY.md
        <key-result-id>/
          README.md
          ACTIVITY.md
          MEASUREMENTS.csv
      ...
    ...
  ...
areas
  README.md
  AGENTS.md
  CLAUDE.md
  ACTIVITY.md
  <area-id>/
    README.md
    ACTIVITY.md
    ...
  <area-2-id>/
    README.md
    ACTIVITY.md
    <sub-area-id>/
      README.md
      ACTIVITY.md
      some-sub-dir/
    ...
resouces/
  README.md
  AGENTS.md
  CLAUDE.md
  ACTIVITY.md
  <resource-area-id>/
    README.md
    untracked-resource-area-dir/
archive/
  projects/
    <archived-project-id>/
      README.md
      ACTIVITY.md
      ...
    ...
  areas/
    <archived-area-id>/
      README.md
      ACTIVITY.md
      ...
    <archived-sub-area-parent-id>/
      <archived-sub-area-id>/
        README.md
        ACTIVITY.md
        ...
    <archived-area-that-was-parent-to-other-sub-areas>/
      <archived-sub-area-2-id>/
        README.md
        ACTIVITY.md
        ...
      <archived-sub-area-3-id>/
        README.md
        ACTIVITY.md
        ...
  resources/
     <archived-resource-area-id>/
      README.md
      ACTIVITY.md
      ...
    <archived-sub-resource-area-parent-id>/
      <archived-sub-resource-area-id>/
        README.md
        ACTIVITY.md
        ...
    <archived-resource-area-that-was-parent-to-other-sub-resource-areas>/
      <archived-sub-resource-area-2-id>/
        README.md
        ACTIVITY.md
        ...
      <archived-sub-resource-area-3-id>/
        README.md
        ACTIVITY.md
        ...

## notes

- all READMEs, ACTIVITYs, AGENTS, and CLAUDE files shown here are managed by the para cli tool. on any change, they are overwritten based on the data and logs under .para/

- if a child area or resource directory is archived, the area or resource directory is moved to the archive with a path that represents the parent path at the time of archival.

- para only manages AGENTS.md and CLAUDE.md files directly under the root or under the projects, areas, resources, archive directories.

- use skills whenever possible

- rules and AGENTS.md should be kept as small as possible. common recommendation is to use a rule to reference skills that the model should use in particular scenarios.

- 
