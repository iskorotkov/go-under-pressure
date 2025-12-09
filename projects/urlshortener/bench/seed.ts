import 'dotenv/config';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const COUNT = Number(process.env.COUNT) || 100000;
const OUTPUT = process.env.OUTPUT || 'codes.json';
const BATCH_SIZE = Number(process.env.BATCH_SIZE) || 5000;

interface CreateResponse {
  short_code: string;
  short_url: string;
  original_url: string;
}

interface BatchResponse {
  urls: CreateResponse[];
}

async function createBatch(startIndex: number, count: number): Promise<string[]> {
  const urls = Array.from({ length: count }, (_, i) => `https://example.com/seed/${startIndex + i}`);

  const res = await fetch(`${BASE_URL}/api/v1/urls/batch`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ urls }),
  });

  if (!res.ok) {
    console.error(`Failed to create batch at ${startIndex}: ${res.status}`);
    return [];
  }

  const data: BatchResponse = await res.json();
  return data.urls.map((u) => u.short_code);
}

async function main() {
  console.log(`Seeding ${COUNT} URLs to ${BASE_URL} (batch size: ${BATCH_SIZE})...`);

  const codes: string[] = [];

  for (let i = 0; i < COUNT; i += BATCH_SIZE) {
    const batchCount = Math.min(BATCH_SIZE, COUNT - i);
    const results = await createBatch(i, batchCount);
    codes.push(...results);
    console.log(`Progress: ${codes.length}/${COUNT}`);
  }

  await Bun.write(OUTPUT, JSON.stringify(codes, null, 2));
  console.log(`Saved ${codes.length} codes to ${OUTPUT}`);
}

main();
