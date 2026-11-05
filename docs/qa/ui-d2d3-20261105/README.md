# UI-D2 / UI-D3 live reproduction (stack 3, 2026-11-05)

Commands (qa_admin token, API http://127.0.0.1:38082/api/v1), observed HTTP codes:
- GET /identities -> 404 ("404 page not found"); GET /smb/identities -> 200 `[]`
- GET /entities/35, /entities/35/duplicates, /recommendations/similar/35 -> 404; GET /entities/3, /entities/4 -> 200
- GET /media/search ids: 29,37,8,55,200,... (file ids); GET /entities ids: 1..20; GET /entities/17/files -> FileID 55
  => /media lists file ids, /entities uses a separate id space; /media never links to /entity.

RED: red_entitydetail.txt (2 failing before fix), red_identitiesapi.txt (1 failing before fix).
