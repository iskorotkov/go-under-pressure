import 'dotenv/config';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const COUNT = Number(process.env.COUNT) || 1000;
const OUTPUT = process.env.OUTPUT || 'codes.json';

interface CreateResponse {
  short_code: string;
  short_url: string;
  original_url: string;
}

async function createUrl(index: number): Promise<string | null> {
  const res = await fetch(`${BASE_URL}/api/v1/urls`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url: `https://example.com/seed/${index}` }),
  });

  if (!res.ok) {
    console.error(`Failed to create URL ${index}: ${res.status}`);
    return null;
  }

  const data: CreateResponse = await res.json();
  return data.short_code;
}

async function main() {
  console.log(`Seeding ${COUNT} URLs to ${BASE_URL}...`);

  const codes: string[] = [];
  const batchSize = 50;

  for (let i = 0; i < COUNT; i += batchSize) {
    const batch = Array.from({ length: Math.min(batchSize, COUNT - i) }, (_, j) =>
      createUrl(i + j)
    );
    const results = await Promise.all(batch);
    codes.push(...results.filter((c): c is string => c !== null));
    console.log(`Progress: ${codes.length}/${COUNT}`);
  }

  await Bun.write(OUTPUT, JSON.stringify(codes, null, 2));
  console.log(`Saved ${codes.length} codes to ${OUTPUT}`);
}

main();
