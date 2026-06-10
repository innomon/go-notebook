const fs = require('fs');
const path = require('path');
const https = require('https');

// Load en-US
const enUsPath = path.resolve(__dirname, 'en-US/index.ts');
const enUsContent = fs.readFileSync(enUsPath, 'utf8');

// Convert ESM to CJS for easy loading
const cjsContent = enUsContent
  .replace('export const enUS =', 'module.exports =')
  .replace(/export {[\s\S]*}/, '');

const tempFile = path.join(__dirname, 'en-us-temp.js');
fs.writeFileSync(tempFile, cjsContent);
const enUS = require(tempFile);
fs.unlinkSync(tempFile);

// Helper to check if a value is a plain object
function isObject(val) {
  return val && typeof val === 'object' && !Array.isArray(val);
}

// Extract all leaf strings and their paths
const leaves = [];
function walk(obj, currentPath = []) {
  for (const key of Object.keys(obj)) {
    const val = obj[key];
    const newPath = [...currentPath, key];
    if (isObject(val)) {
      walk(val, newPath);
    } else if (typeof val === 'string') {
      leaves.push({ path: newPath, val });
    }
  }
}
walk(enUS);

console.log(`Total keys to translate: ${leaves.length}`);

// Function to call translation API using POST
function translateBatch(texts, targetLang) {
  return new Promise((resolve, reject) => {
    // Protect variables: e.g. {count}, {{models}}, etc.
    const varMap = [];
    const protectedTexts = texts.map(text => {
      let protectedText = text;
      // Protect {{...}}
      protectedText = protectedText.replace(/\{\{[^{}]+\}\}/g, match => {
        const id = `__VAR_DBL_${varMap.length}__`;
        varMap.push({ id, original: match });
        return id;
      });
      // Protect {...}
      protectedText = protectedText.replace(/\{[^{}]+\}/g, match => {
        const id = `__VAR_SGL_${varMap.length}__`;
        varMap.push({ id, original: match });
        return id;
      });
      return protectedText;
    });

    const combinedText = protectedTexts.join('\n');

    const body = new URLSearchParams();
    body.append('q', combinedText);

    const targetUrl = `https://translate.googleapis.com/translate_a/single?client=gtx&sl=en&tl=${targetLang}&dt=t`;
    const parsedUrl = new URL(targetUrl);

    const options = {
      hostname: parsedUrl.hostname,
      path: parsedUrl.pathname + parsedUrl.search,
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Content-Length': Buffer.byteLength(body.toString())
      }
    };

    const req = https.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const parsed = JSON.parse(data);
          const segments = parsed[0];
          if (!segments) {
            return reject(new Error('Invalid response structure: ' + data));
          }
          
          let translations = segments.map(seg => seg[0].replace(/\n$/, '').trim());
          
          // If the number of translations doesn't match the texts length,
          // try to split the combined string by newline
          if (translations.length !== texts.length) {
            const combinedTrans = segments.map(seg => seg[0]).join('');
            const lines = combinedTrans.split('\n').map(l => l.trim());
            // Filter out empty lines if they were added
            if (lines.length === texts.length) {
              translations = lines;
            } else {
              console.warn(`Warning: translation count mismatch for ${targetLang}. Expected ${texts.length}, got ${translations.length}. Falling back to original strings for unmatched entries.`);
              // Fallback: pad with original values
              while (translations.length < texts.length) {
                translations.push(texts[translations.length]);
              }
            }
          }

          // Restore variables
          translations = translations.map(trans => {
            let restored = trans;
            for (let i = varMap.length - 1; i >= 0; i--) {
              const { id, original } = varMap[i];
              restored = restored.replace(new RegExp(id, 'g'), original);
              restored = restored.replace(new RegExp(id.toLowerCase(), 'g'), original);
            }
            return restored;
          });

          resolve(translations);
        } catch (err) {
          reject(new Error(`Failed to parse translation response: ${err.message}. Data: ${data}`));
        }
      });
    });

    req.on('error', reject);
    req.write(body.toString());
    req.end();
  });
}

// Translate and build the output object
async function translateLanguage(targetCode, dateFnsCode) {
  console.log(`Translating to ${targetCode}...`);
  const result = JSON.parse(JSON.stringify(enUS)); // deep clone

  // Batch requests in sizes of 50
  const batchSize = 50;
  for (let i = 0; i < leaves.length; i += batchSize) {
    const batch = leaves.slice(i, i + batchSize);
    const texts = batch.map(leaf => leaf.val);
    try {
      const translations = await translateBatch(texts, dateFnsCode);
      for (let j = 0; j < batch.length; j++) {
        const leaf = batch[j];
        const transVal = translations[j] || leaf.val;
        // Set back in result
        let current = result;
        for (let k = 0; k < leaf.path.length - 1; k++) {
          current = current[leaf.path[k]];
        }
        current[leaf.path[leaf.path.length - 1]] = transVal;
      }
      console.log(`Progress: ${Math.min(i + batchSize, leaves.length)}/${leaves.length}`);
      // Sleep 200ms to be polite
      await new Promise(r => setTimeout(r, 200));
    } catch (err) {
      console.error(`Error translating batch at index ${i}:`, err);
      throw err;
    }
  }

  // Write ESM file
  const variableName = targetCode.replace('-', '');
  let output = `export const ${variableName} = ${JSON.stringify(result, null, 2)};\n`;
  const targetDir = path.resolve(__dirname, targetCode);
  if (!fs.existsSync(targetDir)) {
    fs.mkdirSync(targetDir);
  }
  fs.writeFileSync(path.join(targetDir, 'index.ts'), output);
  console.log(`Successfully wrote ${targetCode}/index.ts`);
}

// Run for a target language if requested via command line arg
const args = process.argv.slice(2);
const targetCode = args[0] || 'mr-IN';
const dateFnsCode = args[1] || 'mr';

translateLanguage(targetCode, dateFnsCode)
  .then(() => process.exit(0))
  .catch(err => {
    console.error(err);
    process.exit(1);
  });
