-- CALCITE-7377 as gadak mirrors it: the fix commit is cited in a comment, and
-- CALCITE-7890 links to it and mentions it in its description.
insert into sources(id, kind, base_url) values ('jira', 'jira', 'https://issues.apache.org/jira');

insert into items(id, source_id, kind, external_id, key, title, body_text, author, url, created_at, updated_at) values
  ('jira:13639034', 'jira', 'issue', '13639034', 'CALCITE-7377',
   'Validator should reject a DESCRIPTOR in a table function when it is not an identifier',
   'SELECT * FROM TABLE(TUMBLE(TABLE orders, DESCRIPTOR(1 + 2), INTERVAL ''1'' MINUTE)) crashes.',
   'Mihai Budiu', 'https://issues.apache.org/jira/browse/CALCITE-7377',
   '2026-01-14T19:53:03.000Z', '2026-05-31T14:37:29.552Z'),
  ('jira:13650001', 'jira', 'issue', '13650001', 'CALCITE-7890',
   'SqlDescriptorOperator.checkOperandTypes rejects simple identifiers and is never called',
   'TUMBLE, HOP and SESSION check their DESCRIPTOR operands in SqlWindowTableFunction.validateColumnNames, which CALCITE-7377 added.',
   'Vladimir Sitnikov', 'https://issues.apache.org/jira/browse/CALCITE-7890',
   '2026-10-20T10:00:00.000Z', '2026-10-20T10:00:00.000Z');

insert into issues_raw(item_id, key, project_key, issue_type, status, status_category, priority, assignee, reporter,
                       labels, components, fix_versions, resolution, created_at, updated_at, resolved_at) values
  ('jira:13639034', 'CALCITE-7377', 'CALCITE', 'Bug', 'Closed', 'done', 'Minor', 'Mihai Budiu', 'Mihai Budiu',
   '["pull-request-available"]', '["core"]', '["1.42.0"]', 'Fixed',
   '2026-01-14T19:53:03.000Z', '2026-05-31T14:37:29.552Z', '2026-05-31T14:37:29.552Z'),
  ('jira:13650001', 'CALCITE-7890', 'CALCITE', 'Bug', 'Open', 'new', 'Major', null, 'Vladimir Sitnikov',
   '[]', '["core"]', '[]', null,
   '2026-10-20T10:00:00.000Z', '2026-10-20T10:00:00.000Z', null);

insert into comments(id, item_id, author, body_text, created_at) values
  ('1', 'jira:13639034', 'Julian Hyde',
   'It''s not intended behavior that 1 + 2 should be a valid descriptor.', '2026-01-14T20:10:00.000Z'),
  ('2', 'jira:13639034', 'Mihai Budiu',
   'Fixed in [https://github.com/apache/calcite/commit/fd7220017887c96a46ff604a67954242a67b1513]

Thank you for the reviews [~dmsysolyatin] [~jensen]', '2026-05-30T08:00:00.000Z');

insert into links(item_id, type, direction, target_key) values
  ('jira:13650001', 'Reference', 'outward', 'CALCITE-7377'),
  ('jira:13639034', 'Reference', 'inward', 'CALCITE-7890');
