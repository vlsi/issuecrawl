-- apache/calcite as gitcrawl mirrors it after `gitcrawl sync apache/calcite
-- --state all --with pr-metadata`: pull request 4746 carries the key in its
-- title, and its details hold the merge commit that CALCITE-7377 cites.
insert into repositories(id, owner, name, full_name, github_repo_id, raw_json, updated_at) values
  (2, 'apache', 'calcite', 'apache/calcite', '22264881', '{}', '2026-10-26T00:00:00Z');

insert into threads(id, repo_id, github_id, number, kind, state, title, body, author_login, html_url,
                    labels_json, assignees_json, raw_json, content_hash,
                    created_at_gh, updated_at_gh, closed_at_gh, merged_at_gh, updated_at) values
  (4746, 2, '3170000001', 4746, 'pull_request', 'closed',
   '[CALCITE-7377] Validator should reject a DESCRIPTOR in a table function when it is not an identifier',
   'Fixes https://issues.apache.org/jira/browse/CALCITE-7377', 'mihaibudiu',
   'https://github.com/apache/calcite/pull/4746',
   '[{"name":"LGTM-will-merge-soon"}]', '[]', '{}', 'h4746',
   '2026-01-15T01:00:00Z', '2026-05-30T07:59:00Z', '2026-05-30T07:58:00Z', '2026-05-30T07:58:00Z',
   '2026-10-26T00:00:00Z');

insert into pull_request_details(thread_id, repo_id, number, raw_json, fetched_at, updated_at) values
  (4746, 2, 4746, '{"merge_commit_sha":"fd7220017887c96a46ff604a67954242a67b1513","merged":true}',
   '2026-10-26T00:00:00Z', '2026-10-26T00:00:00Z');

insert into sync_runs(repo_id, scope, status, started_at, finished_at) values
  (2, 'all', 'success', '2026-10-26T00:00:00Z', '2026-10-26T00:10:00Z');
