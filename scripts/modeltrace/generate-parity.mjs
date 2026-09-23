// Synthetic fixtures check implementation parity, not model identification accuracy.
import { readFileSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';
const [corePath, bankPath, outputPath] = process.argv.slice(2);
if (!corePath || !bankPath || !outputPath) throw new Error('Usage: node generate-parity.mjs CORE BANK OUTPUT');
const { analyzeGlobalOutputs } = await import(pathToFileURL(resolve(corePath)).href);
const bank = JSON.parse(readFileSync(bankPath, 'utf8'));
const sequence = (n, seed) => { let x = seed; return Array.from({ length: n }, () => { x = (Math.imul(x, 1664525) + 1013904223) >>> 0; return x % 355 + 1; }); };
const cases = [1, 2, 3].map(n => ({ name: `${n}-responses`, outputs: Array.from({ length: n }, (_, i) => ({ text: 'Explanation words. [' + sequence(292 + i * 13, 123 + i * 789).join(',') + '] end.', expected_count: 300 })) }));
cases.push({ name: 'reject-short-output', outputs: [{ text: '1,2,3', expected_count: 300 }, { text: sequence(310, 98).join(' '), expected_count: 310 }] });
const results = cases.map(c => ({ ...c, expected: analyzeGlobalOutputs(c.outputs, bank) }));
writeFileSync(outputPath, JSON.stringify(results, null, 2) + '\n');
