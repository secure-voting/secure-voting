import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { suffix } from "../src/ids.js";

type DatasetItem = {
  id: string;
  name: string;
  format: "approval" | "ranking" | "score" | string;
  candidates?: Array<{ id?: string; name?: string }>;
};

type DatasetImportResponse = {
  id: string;
};

async function importDataset(
  api: Awaited<ReturnType<typeof createApiClient>>,
  token: string,
  filename: string,
  content: string,
  name: string
): Promise<DatasetItem> {
  const multipart = {
    name,
    description: `external import ${name}`,
    file: {
      name: filename,
      mimeType: "text/plain",
      buffer: Buffer.from(content, "utf-8"),
    },
  };

  const created = await api.postMultipart<DatasetImportResponse>(
    "/datasets/import",
    multipart,
    token
  );

  expect(created.id).toBeTruthy();

  const dataset = await api.get<DatasetItem>(`/datasets/${created.id}`, token);
  expect(dataset.id).toBe(created.id);

  return dataset;
}

test.describe("external dataset formats import", () => {
  test("researcher can import PrefLib SOC and SOI plus Pabulib PB formats", async () => {
    const api = await createApiClient();

    try {
      const researcher = await api.login(
        process.env.RESEARCHER_EMAIL || process.env.BOOTSTRAP_RESEARCHER_EMAIL || "researcher@example.com",
        process.env.RESEARCHER_PASSWORD || process.env.BOOTSTRAP_RESEARCHER_PASSWORD || "ResearcherPass123!"
      );

      const sfx = suffix();

      const soc = `# FILE NAME: sample.soc
# TITLE: Sample PrefLib SOC
# NUMBER ALTERNATIVES: 3
# NUMBER VOTERS: 3
# NUMBER UNIQUE ORDERS: 2
# ALTERNATIVE NAME 1: Alice
# ALTERNATIVE NAME 2: Bob
# ALTERNATIVE NAME 3: Carol
2: 1,2,3
1: 2,1,3
`;

      const soi = `# FILE NAME: sample.soi
# TITLE: Sample PrefLib SOI
# NUMBER ALTERNATIVES: 3
# NUMBER VOTERS: 2
# NUMBER UNIQUE ORDERS: 2
# ALTERNATIVE NAME 1: Alice
# ALTERNATIVE NAME 2: Bob
# ALTERNATIVE NAME 3: Carol
1: 1,3,2
1: 3,2,1
`;

      const pbApproval = `META
key; value
description; PB approval sample
vote_type; approval
PROJECTS
project_id; name
1; Alpha
2; Beta
3; Gamma
VOTES
voter_id; vote
1; 1,2
2; 2,3
`;

      const pbOrdinal = `META
key; value
description; PB ordinal sample
vote_type; ordinal
PROJECTS
project_id; name
1; Alpha
2; Beta
3; Gamma
VOTES
voter_id; vote
1; 2,1,3
`;

      const pbScoring = `META
key; value
description; PB scoring sample
vote_type; scoring
PROJECTS
project_id; name
1; Alpha
2; Beta
3; Gamma
VOTES
voter_id; vote
1; 1=5,2=3,3=1
`;

      const cases = [
        {
          filename: `preflib-${sfx}.soc`,
          content: soc,
          name: `PrefLib SOC ${sfx}`,
          format: "ranking",
        },
        {
          filename: `preflib-${sfx}.soi`,
          content: soi,
          name: `PrefLib SOI ${sfx}`,
          format: "ranking",
        },
        {
          filename: `pabulib-approval-${sfx}.pb`,
          content: pbApproval,
          name: `Pabulib approval ${sfx}`,
          format: "approval",
        },
        {
          filename: `pabulib-ordinal-${sfx}.pb`,
          content: pbOrdinal,
          name: `Pabulib ordinal ${sfx}`,
          format: "ranking",
        },
        {
          filename: `pabulib-scoring-${sfx}.pb`,
          content: pbScoring,
          name: `Pabulib scoring ${sfx}`,
          format: "score",
        },
      ];

      for (const item of cases) {
        const dataset = await importDataset(
          api,
          researcher.accessToken,
          item.filename,
          item.content,
          item.name
        );

        expect(dataset.name).toBe(item.name);
        expect(dataset.format).toBe(item.format);
        expect(dataset.candidates?.length).toBe(3);
      }
    } finally {
      await api.dispose();
    }
  });
});