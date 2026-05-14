import type {
  ExpectedArtifact,
  ExpectedArtifactsTableProps,
  PublishedArtifact,
} from "../types";
import { BylineLine } from "./BylineLine";
import { canonicalizeByline } from "./common";

function expectedAuthorLine(artifact: ExpectedArtifact): string | null {
  return artifact.expected_author_line ?? artifact.author_line ?? null;
}

function artifactKey(artifact: Pick<ExpectedArtifact | PublishedArtifact, "path">): string {
  return artifact.path;
}

function actualByPath(actualArtifacts: PublishedArtifact[]): Map<string, PublishedArtifact> {
  return new Map(actualArtifacts.map((artifact) => [artifactKey(artifact), artifact]));
}

function statusLabel(expected: ExpectedArtifact, actual: PublishedArtifact | undefined): string {
  if (!actual) return expected.required === false ? "missing - optional" : "missing - required";
  const expectedLine = canonicalizeByline(expectedAuthorLine(expected));
  const actualLine = canonicalizeByline(actual.author_line);
  if (expectedLine && actualLine && expectedLine !== actualLine) {
    return "byline drift";
  }
  return "published";
}

function publishRecipe(artifact: ExpectedArtifact): string {
  return [
    "striatum publish-artifact",
    `--path ${artifact.path}`,
    `--kind ${artifact.kind}`,
    `--logical-name ${artifact.logical_name}`,
  ].join(" ");
}

export function ExpectedArtifactsTable({
  expectedArtifacts,
  actualArtifacts,
  session,
  lease,
}: ExpectedArtifactsTableProps) {
  const actual = actualByPath(actualArtifacts);

  return (
    <table
      className="expected-artifacts-table"
      data-component="ExpectedArtifactsTable"
      aria-label="expected artifacts"
      data-session-id={session?.session_id}
      data-lease-id={lease?.lease_id}
    >
      <thead>
        <tr>
          <th scope="col">logical_name</th>
          <th scope="col">kind</th>
          <th scope="col">path</th>
          <th scope="col">required</th>
          <th scope="col">expected_author_line</th>
          <th scope="col">status</th>
        </tr>
      </thead>
      <tbody>
        {expectedArtifacts.map((expected) => {
          const published = actual.get(expected.path);
          const label = statusLabel(expected, published);
          const bylineDrift = label === "byline drift";
          const missingRequired = label === "missing - required";

          return (
            <tr key={`${expected.logical_name}:${expected.path}`}>
              <td>
                <code>{expected.logical_name}</code>
              </td>
              <td>
                <span
                  className={`artifact-kind-chip artifact-kind-chip--${expected.kind}`}
                  aria-label={`artifact kind: ${expected.kind}`}
                >
                  {expected.kind}
                </span>
              </td>
              <td>
                <code>{expected.path}</code>
              </td>
              <td>{expected.required === false ? "optional" : "required"}</td>
              <td>
                <BylineLine authorLine={expectedAuthorLine(expected)} />
              </td>
              <td>
                <span className={`artifact-status artifact-status--${label.replace(/\s+/g, "-")}`}>
                  {label}
                </span>
                {published?.sha256 ? (
                  <small>
                    sha256 <code>{published.sha256}</code>
                  </small>
                ) : null}
                {published ? (
                  <BylineLine
                    authorLine={published.author_line ?? null}
                    expectedAuthorLine={expectedAuthorLine(expected)}
                    attested={published.lane_attestation_chip?.attested}
                  />
                ) : null}
                {missingRequired ? (
                  <code className="artifact-recipe">{publishRecipe(expected)}</code>
                ) : null}
                {bylineDrift ? <small>actual byline differs from expected byline</small> : null}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
