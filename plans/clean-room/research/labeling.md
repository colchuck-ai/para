# Research: sensitivity, provenance and trust labeling

Gathered 2026-10-22 for `spec.md` chapter 6. Items marked **unverified** were not checked against a
primary source.

## Sensitivity schemes and inheritance

- **TLP 2.0** ([FIRST](https://www.first.org/tlp/)) is a set of sharing rules, not a classification
  scheme. RED: named individuals only. AMBER+STRICT: the organisation only. AMBER: the organisation
  and its clients. GREEN: the community but not public channels. CLEAR: anyone. Wider sharing needs
  the source's permission, and labels must stay in their original form. It defines no defaults, no
  inheritance and no rule for derived content.
- **Microsoft Purview** ([labels](https://learn.microsoft.com/en-us/purview/sensitivity-labels),
  [library default](https://learn.microsoft.com/en-us/purview/sensitivity-labels-sharepoint-default-label)):
  labels are ordered by priority, and each item carries exactly one, stored in the file. A policy
  default covers unlabelled items, and mandatory labelling blocks saving without one. Lowering a
  label needs a justification, which is audited. A library default overrides a lower automatic
  label but never a manual one, and does not touch existing files. Site and group labels do not
  label their items. When Copilot combines items, it shows the highest-priority label among them.
- **FIPS 199** ([PDF](https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.199.pdf)): a system holding
  several information types takes the high-water mark. Management may raise the result.
- **ISO/IEC 27001:2022** A.5.12/5.13 only require classifying and labelling procedures (**unverified**:
  paywalled). **SP 800-60** was not fetched.

## Information-flow models

- **Bell–LaPadula** (confidentiality: no read up, no write down) and **Biba** (its integrity dual).
  Biba's **low-water mark** lowers integrity to the minimum after a write or read, which is exactly
  "trust only falls through derivation". Its known flaw is **label creep**: everything drifts to the
  lowest level. (**Unverified** against Biba 1977; source:
  [Purdue lecture notes](https://www.cs.purdue.edu/homes/ninghui/courses/Spring18/handouts/06_integrity.pdf).)
- **CaMeL** ([arXiv 2503.18813](https://arxiv.org/html/2503.18813)): a privileged model plans from the
  trusted query and never sees untrusted data; a quarantined model parses that data and has no tools.
  Every value carries its provenance and its allowed readers, derived values carry their inputs'
  dependencies, and policies are checked when a tool is called.
- **FIDES** ([arXiv 2505.23643](https://arxiv.org/abs/2505.23643)): an agent planner with
  confidentiality and integrity labels and dynamic taint tracking (**unverified** beyond the abstract).

## Provenance models

- **W3C PROV-DM** ([spec](https://www.w3.org/TR/prov-dm/)): Entity, Activity and Agent, related by
  wasGeneratedBy, used, wasDerivedFrom, wasAttributedTo and wasAssociatedWith. Derivation has the
  subtypes Revision, Quotation and Primary Source ("produced by some agent with direct experience").
- **C2PA** ([spec 2.2](https://spec.c2pa.org/specifications/specifications/2.2/specs/C2PA_Specification.html)):
  a signed manifest of actions and ingredients. A hard binding (a hash of the asset) detects any later
  change, and a new change is appended rather than replacing history. Manifests may be embedded or
  stored separately.
- **IPTC digitalSourceType** ([vocabulary](https://cv.iptc.org/newscodes/digitalsourcetype/)):
  `digitalCapture`, `trainedAlgorithmicMedia`, `compositeWithTrainedAlgorithmicMedia`,
  `algorithmicallyEnhanced`, `humanEdits`. The established terms for "captured or inferred".
- **SLSA v1** ([provenance](https://slsa.dev/spec/v1.0/provenance)): subjects identified by digest,
  with the build type, parameters, builder identity and resolved inputs.
- **OpenLineage** ([object model](https://openlineage.io/docs/spec/object-model)): Job, Run and
  Dataset, with extensible facets.
- **Fields that recur across all of them:** the subject and its content hash; the activity that
  produced it; the inputs and their hashes; the agent or tool, including model and version; a time;
  and the derivation kind (capture, revision, quotation or generation).

## Trust and credibility grading

- **The Admiralty/NATO system** grades two things separately: **source reliability**, A–F, from the
  source's track record (F: cannot be judged); and **information credibility**, 1–6, from
  corroboration (6: cannot be judged). They are separate because a reliable source can pass on false
  information. (**Unverified** against the doctrine, AJP-2.1 and FM 2-22.3; sources:
  [Wikipedia](https://en.wikipedia.org/wiki/Admiralty_code),
  [SANS](https://www.sans.org/blog/enhance-your-cyber-threat-intelligence-with-the-admiralty-system).)
- **Primary, secondary and tertiary sources** measure distance from the event, not reliability.
- No standard was found that grades the trustworthiness of AI-generated content. IPTC and C2PA only
  disclose that content was generated.

## Where file-level metadata lives

| Option | For | Against |
| --- | --- | --- |
| Frontmatter | travels with the file | text files only; editors and agents change or strip it; mixes metadata into content |
| Sidecar file or central index | any file format; owned by the tool | orphaned on rename or move unless the tool handles it |
| Extended attributes | invisible to editors | git does not keep them |
| Directory-scoped config (like `.gitattributes`) | the nearest file wins, which suits inheritance | path patterns drift; a deeper file can loosen a setting |
| Content hash | the standard way to detect a change after labelling | says that a file changed, not what changed |

## Implications for para

**Sensitivity.**
- *Supported:* derived content taking the strictest label of its inputs (FIPS 199, Bell–LaPadula,
  Copilot), and only a person loosening a label (Purview's justification, TLP's permission from the
  source).
- *Para adds what TLP lacks*, so the spec must say so: inheritance, defaults and derivation rules.
- *Missing:* a rule for unlabelled content; a recorded actor and reason for every loosening; a
  defence against label creep; and a mapping from TLP audiences to para's real destinations (the git
  remote, a hosted model API, publishing).

**Provenance.**
- *Supported:* the planned fields match PROV, C2PA and SLSA.
- *Missing:* a content hash binding the record to the bytes; the tool or model identity and version
  for inferred content; review as an event tied to a specific hash; and handling of edits after
  labelling (on a hash mismatch, append a revision attributed to the editor; never silently keep or
  discard the old record).
- *Suggestion:* use the IPTC digitalSourceType terms rather than new ones.

**Trust.**
- *Supported:* deriving trust from provenance (as CaMeL does), and "inference only lowers" (Biba's
  low-water mark).
- *Caution:* "review raises trust" has no place in Biba. It is a separate endorsement step and should
  be recorded like a loosening.
- *Answer to the credibility gap:* use two axes, as the Admiralty system does. **Source reliability**
  is stored once per source as a person's judgment, defaulting to "cannot judge". **Content fidelity**
  is derived from provenance (capture, then human edit, then inference), adjusted by review.
- *Out of scope:* corroboration across independent reports.

**Storage.** Keep sensitivity in directory-scoped files with tighten-only inheritance enforced by
para. Keep provenance in a para-owned sidecar or index keyed by path and hash. Avoid extended
attributes. Use frontmatter only if people editing it is acceptable.
