# Reconcile report 2026-10-07 (tasks.md vs machine evidence)

Rule: tick only with deliverables present + committed green evidence + no open NO-GO/owed independent review (11.4.142, 11.4.226). No clean GO verdict file exists anywhere under evidence/ (no evidence/reviews/), so almost nothing qualifies. Inline markers appended to 50 task lines (no gate parses tasks.md checkboxes; only comment references in scripts).

Ticked: 1. Not ticked: 684.

| task | decision | reason | evidence |
|---|---|---|---|
| T001 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T002 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T003 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T004 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T005 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T005a | not ticked | impl committed b3728d76; open NO-GO review: envelope/pins fixes r4 committed e0ebebc9 | evidence/wp09/ |
| T005b | not ticked | impl committed e9e501ff; open NO-GO review: envelope/pins fixes r4 committed e0ebebc9; commit says part/blocked | evidence/wp09/ |
| T094a | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T005c | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T005d | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T137a | not ticked | impl committed b3728d76; open NO-GO review: envelope/pins fixes r4 committed e0ebebc9 | evidence/wp09/ |
| T006a | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T006 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T007 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T008 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T009 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T010 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T011 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T012 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T013 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T014 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T011a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T015 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T016 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T017 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T018 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T019 | TICKED | deliverables tracked+present; --check clean; green x3 | evidence/wp02/review-fix3-green-*, round5-green-* |
| T020 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T021 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T022 | not ticked | impl committed f04c555d; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp02/ |
| T023 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T024 | not ticked | impl committed f04c555d; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp02/ |
| T025 | not ticked | impl committed f04c555d; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp02/ |
| T026 | not ticked | impl committed f04c555d; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp02/ |
| T027 | not ticked | impl committed f04c555d; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp02/ |
| T028 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T029 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T030 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T031 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T033 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T032 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T034 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T035 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T036 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T037 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T038 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T039 | not ticked | impl committed 11ba4b1d; open NO-GO review: round 3 fixes committed 4b460797 | evidence/wp04/ |
| T040 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T040a | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T040b | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T041 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T042 | not ticked | impl committed 11ba4b1d; open NO-GO review: round 3 fixes committed 4b460797 | evidence/wp04/ |
| T042a | not ticked | impl committed 11ba4b1d; open NO-GO review: round 3 fixes committed 4b460797 | evidence/wp04/ |
| T043 | not ticked | impl committed 11ba4b1d; open NO-GO review: round 3 fixes committed 4b460797 | evidence/wp04/ |
| T044 | not ticked | impl committed 11ba4b1d; open NO-GO review: round 3 fixes committed 4b460797 | evidence/wp04/ |
| T045 | not ticked | impl committed 11ba4b1d; open NO-GO review: round 3 fixes committed 4b460797 | evidence/wp04/ |
| T045a | not ticked | impl committed 11ba4b1d; open NO-GO review: round 3 fixes committed 4b460797 | evidence/wp04/ |
| T046 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T046a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T047 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T048 | not ticked | impl committed 8bc60bd7; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp05/ |
| T048a | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T049 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T050 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T051 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T052 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T053 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T054 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T051a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T055 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T056 | not ticked | impl committed 8bc60bd7; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp05/ |
| T057 | not ticked | impl committed 8bc60bd7; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp05/ |
| T058 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T059 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T060 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T061 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T062 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T063 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T064 | not ticked | impl committed e7a6a9b9; open NO-GO review: register r1 28e3662b | evidence/wp01/ |
| T064a | not ticked | impl committed e7a6a9b9; open NO-GO review: register r1 28e3662b | evidence/wp01/ |
| T065 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T066 | not ticked | impl committed e7a6a9b9; open NO-GO review: register r1 28e3662b | evidence/wp01/ |
| T067 | not ticked | impl committed e7a6a9b9; open NO-GO review: register r1 28e3662b | evidence/wp01/ |
| T067a | not ticked | impl committed e7a6a9b9; open NO-GO review: register r1 28e3662b | evidence/wp01/ |
| T068 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T069 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T070 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T071 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T012a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T072 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T073 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T074 | not ticked | impl committed 7f5e9f4b; independent review owed, no GO verdict recorded | evidence/wp07/ |
| T075 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T076 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T077 | not ticked | blocked on containers/CPA host (progress.yml) | - |
| T078 | not ticked | impl committed 7f5e9f4b; independent review owed, no GO verdict recorded | evidence/wp07/ |
| T079 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T080 | not ticked | impl committed 40a1f8e3; independent review owed, no GO verdict recorded | evidence/wp07/ |
| T081 | not ticked | impl committed 40a1f8e3; independent review owed, no GO verdict recorded | evidence/wp07/ |
| T083 | not ticked | impl committed 40a1f8e3; independent review owed, no GO verdict recorded | evidence/wp07/ |
| T084 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T081a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T085 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T082 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T084a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T086 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T087 | not ticked | in_progress per progress.yml; no task-id-bearing commit or evidence found | - |
| T088 | not ticked | impl committed 87909608; open NO-GO review: round 3 fixes committed 350372a8 | evidence/wp08/ |
| T089 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T089a | not ticked | impl committed e9e501ff; open NO-GO review: envelope/pins fixes r4 committed e0ebebc9; commit says part/blocked | evidence/wp09/ |
| T090 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T091 | not ticked | impl committed 87909608; open NO-GO review: round 3 fixes committed 350372a8 | evidence/wp08/ |
| T087a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T092 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T093 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T093a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T094 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T094d | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T094b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T094c | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T095 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T096 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T097 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T098 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T099 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T100 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T101 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T102 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T103 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T104 | not ticked | impl committed b53be11a; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp09-p1/ |
| T105 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T106 | not ticked | impl committed b53be11a; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp09-p1/ |
| T107 | not ticked | impl committed 960c553a; independent review owed, no GO verdict recorded | evidence/wp09-p1/ |
| T108 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T109 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T110 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T111 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T112 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T113 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T114 | not ticked | impl committed 960c553a; independent review owed, no GO verdict recorded | evidence/wp09-p1/ |
| T115 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T116 | not ticked | impl committed 960c553a; independent review owed, no GO verdict recorded | evidence/wp09-p1/ |
| T117 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T118 | not ticked | impl committed a7cfc6d3; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp09-p1/ |
| T119 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T120 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T121 | not ticked | impl committed a7cfc6d3; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp09-p1/ |
| T121a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T121b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T122 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T123 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T124 | not ticked | impl committed a7cfc6d3; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp09-p1/ |
| T125 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T126 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T127 | not ticked | impl committed 6d5ebb64; open NO-GO review: test-infra fixes c2141c78; commit says part/blocked | evidence/wp12/ |
| T128 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T129 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T130 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T131 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T132 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T133 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T134 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T134a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T135 | not ticked | impl committed 6d5ebb64; open NO-GO review: test-infra fixes c2141c78; commit says part/blocked | evidence/wp12/ |
| T136 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T137 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T138 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T139 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T139a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T140 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T141 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T142 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T143 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T144 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T145 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T146 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T147 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T148 | not ticked | impl committed 960c553a; independent review owed, no GO verdict recorded | evidence/wp09-p1/ |
| T149 | not ticked | impl committed 960c553a; independent review owed, no GO verdict recorded | evidence/wp09-p1/ |
| T150 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T151 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T152 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T153 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T154 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T155 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T156 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T157 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T158 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T159 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T160 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T161 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T162 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T163 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T164 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T165 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T166 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T167 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T168 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T169 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T170 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T171 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T172 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T173 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T174 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T175 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T175a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T176 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T177 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T178 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T179 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T180 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T181 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T182 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T183 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T184 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T185 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T186 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T187 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T188 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T189 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T190 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T191 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T192 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T193 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T194 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T195 | not ticked | impl committed e5f568f6; open NO-GO review: coverage r1 846d441f (r5 convergence note in evidence/wp23); commit says part/blocked | evidence/wp23/ |
| T196 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T197 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T198 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T199 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T200 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T200a | not ticked | impl committed e5f568f6; open NO-GO review: coverage r1 846d441f (r5 convergence note in evidence/wp23); commit says part/blocked | evidence/wp23/ |
| T201 | not ticked | impl committed e5f568f6; open NO-GO review: coverage r1 846d441f (r5 convergence note in evidence/wp23); commit says part/blocked | evidence/wp23/ |
| T202 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T203 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T204 | not ticked | impl committed e5f568f6; open NO-GO review: coverage r1 846d441f (r5 convergence note in evidence/wp23); commit says part/blocked | evidence/wp23/ |
| T205 | not ticked | impl committed e5f568f6; open NO-GO review: coverage r1 846d441f (r5 convergence note in evidence/wp23); commit says part/blocked | evidence/wp23/ |
| T206 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T207 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T208 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T209 | not ticked | impl committed 34db9504; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp24/ |
| T210 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T211 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T212 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T213 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T214 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T215 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T216 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T217 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T218 | not ticked | impl committed 34db9504; independent review owed, no GO verdict recorded; commit says part/blocked | evidence/wp24/ |
| T219 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T220 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T221 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T222 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T223 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T224 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T225 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T225e | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T225d | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T225a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T225b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T225c | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T226 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T227 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T228 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T229 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T230 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T231 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T232 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T233 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T234 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T235 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T236 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T237 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T238 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T239 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T240 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T240a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T241 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T241a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T242 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T243 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T244 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T245 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T246 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T247 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T248 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T248a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T249 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T250 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T251 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T252 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T253 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T254 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T254a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T254b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T255 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T256 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T257 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T258 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T259 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T260 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T261 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T262 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T263 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T264 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T265 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T266 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T267 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T268 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T269 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T270 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T271 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T272 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T272a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T273 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T274 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T275 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T276 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T277 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T277a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T277b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T278 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T279 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T280 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T281 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T282 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T283 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T283a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T284 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T285 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T286 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T287 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T288 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T288a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T288b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T289 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T290 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T291 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T292 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T293 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T294 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T295 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T296 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T297 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T298 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T299 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T300 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T301 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T302 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T303 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T304 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T305 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T306 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T307 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T308 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T308a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T309 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T310 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T311 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T312 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T313 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T314 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T315 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T316 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T317 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T318 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T318a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T319 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T320 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T321 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T322 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T322a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T323 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T324 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T325 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T326 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T327 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T328 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T329 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T330 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T331 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T332 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T333 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T334 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T334a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T335 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T335a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T336 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T336a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T337 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T338 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T339 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T340 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T341 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T342 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T343 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T344 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T345 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T346 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T347 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T348 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T349 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T350 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T351 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T352 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T353 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T354 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T355 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T356 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T357 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T358 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T359 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T360 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T361 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T362 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T363 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T364 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T365 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T366 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T367 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T368 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T369 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T370 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T371 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T372 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T373 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T374 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T375 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T376 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T377 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T378 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T379 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T380 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T381 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T382 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T383 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T384 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T385 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T386 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T387 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T388 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T389 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T390 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T391 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T392 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T393 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T394 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T394a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T395 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T396 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T397 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T398 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T399 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T400 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T401 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T402 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T403 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T404 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T405 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T406 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T407 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T408 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T409 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T410 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T411 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T412 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T413 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T414 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T415 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T416 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T417 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T418 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T419 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T420 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T420a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T421 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T422 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T423 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T424 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T425 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T426 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T427 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T428 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T429a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T429 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T430 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T431 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T432 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T433 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T434 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T434a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T435 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T435a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T435c | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T435b | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T436 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T437 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T438 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T439 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T440 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T440a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T440b | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T441 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T442a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T442b | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T440c | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T442 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T443 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T444 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T445 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T446 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T447 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T447a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T447b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T448 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T448a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T449 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T450 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T451 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T452 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T453 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T454 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T455 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T456 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T456a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T456b | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T456c | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T457 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T458 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T459 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T460 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T461 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T462 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T462a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T463 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T464 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T465 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T466 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T467 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T468 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T469 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T470 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T471 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T472 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T473 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T474 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T475 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T476 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T477 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T478 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T479 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T480 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T481 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T482 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T483 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T484 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T485 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T486 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T487 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T488 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T489 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T490 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T491 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T492 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T493 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T494 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T495 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T496 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T497 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T498 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T499 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T500 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T501 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T502 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T503 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T504 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T504a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T505 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T506 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T507 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T508 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T509 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T510 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T511 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T512 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T513 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T514 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T515 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T516 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T517 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T518 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T519 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T520 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T521 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T522 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T523 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T524 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T525 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T526 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T527 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T528 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T529 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T530 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T530a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T531 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T532 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T533 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T534 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T535 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T536 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T537 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T538 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T539 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T540 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T541 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T542 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T543 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T544 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T545 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T546 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T547 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T548 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T548a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T549 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T550 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T551 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T552 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T553 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T554 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T555 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T556 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T557 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T558 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T559 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T560 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T561 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T561a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T562 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T563 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T564 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T564a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T565 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T566 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T566a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T567 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T568 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T569 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T570 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T570a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T571 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T572 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T573 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T574 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T574a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T575 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T576 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T577 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T578 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T579 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T579a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T580 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T580e | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T580f | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T580b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T580a | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T580d | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T580c | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T581 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T582 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T583 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T584 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T585 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T586 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T587 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T588 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T589 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T590 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T591 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T592 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T593 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T594 | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T595 | not ticked | [REVIEW] task: no verdict file under evidence/reviews exists | - |
| T595a | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
| T595b | not ticked | no implementing commit or evidence found (not started or unmapped) | - |
