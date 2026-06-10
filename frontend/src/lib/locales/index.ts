import { enUS } from './en-US';
import { bnIN } from './bn-IN';
import { hiIN } from './hi-IN';
import { mrIN } from './mr-IN';
import { teIN } from './te-IN';
import { taIN } from './ta-IN';
import { guIN } from './gu-IN';
import { knIN } from './kn-IN';
import { urIN } from './ur-IN';

export const resources = {
  'en-US': { translation: enUS },
  'en-IN': { translation: enUS },
  'en-GB': { translation: enUS },
  'hi-IN': { translation: hiIN },
  'bn-IN': { translation: bnIN },
  'bn-BD': { translation: bnIN },
  'mr-IN': { translation: mrIN },
  'te-IN': { translation: teIN },
  'ta-IN': { translation: taIN },
  'gu-IN': { translation: guIN },
  'ur-PK': { translation: urIN },
  'ur-IN': { translation: urIN },
  'kn-IN': { translation: knIN },
  'or-IN': { translation: hiIN },
  'ml-IN': { translation: hiIN },
  'pa-IN': { translation: hiIN },
  'as-IN': { translation: hiIN },
  'mai-IN': { translation: hiIN },
  'sat-IN': { translation: hiIN },
  'ks-IN': { translation: hiIN },
  'ne-NP': { translation: hiIN },
  'sd-PK': { translation: hiIN },
  'kok-IN': { translation: hiIN },
  'doi-IN': { translation: hiIN },
  'mni-IN': { translation: hiIN },
  'brx-IN': { translation: hiIN },
  'bho-IN': { translation: hiIN },
  'hne-IN': { translation: hiIN },
  'mag-IN': { translation: hiIN },
  'mwr-IN': { translation: hiIN },
  'awa-IN': { translation: hiIN },
  'lus-IN': { translation: enUS },
  'kha-IN': { translation: enUS },
  'grt-IN': { translation: enUS },
  'trp-IN': { translation: enUS },
  'tcy-IN': { translation: knIN },
  'anp-IN': { translation: hiIN },
  'mtr-IN': { translation: hiIN },
  'bfy-IN': { translation: hiIN },
  'bns-IN': { translation: hiIN },
} as const;

export type TranslationKeys = typeof enUS;

export type LanguageCode = keyof typeof resources;

export type Language = {
  code: LanguageCode;
  label: string;
};

export const languages: Language[] = [
  { code: 'en-US', label: 'English (US)' },
  { code: 'en-IN', label: 'English (India)' },
  { code: 'en-GB', label: 'English (UK)' },
  { code: 'hi-IN', label: 'हिन्दी (Hindi)' },
  { code: 'bn-IN', label: 'বাংলা (Bengali - India)' },
  { code: 'bn-BD', label: 'বাংলা (Bangla - Bangladesh)' },
  { code: 'mr-IN', label: 'मराठी (Marathi)' },
  { code: 'te-IN', label: 'తెలుగు (Telugu)' },
  { code: 'ta-IN', label: 'தமிழ் (Tamil)' },
  { code: 'gu-IN', label: 'ગુજરાતી (Gujarati)' },
  { code: 'ur-PK', label: 'اردو (Urdu - Pakistan)' },
  { code: 'ur-IN', label: 'اردو (Urdu - India)' },
  { code: 'kn-IN', label: 'ಕನ್ನಡ (Kannada)' },
  { code: 'or-IN', label: 'ଓଡ଼ିଆ (Odia)' },
  { code: 'ml-IN', label: 'മലയാളം (Malayalam)' },
  { code: 'pa-IN', label: 'ਪੰਜਾਬੀ (Punjabi)' },
  { code: 'as-IN', label: 'অসমীয়া (Assamese)' },
  { code: 'mai-IN', label: 'मैथिली (Maithili)' },
  { code: 'sat-IN', label: 'संताली (Santali)' },
  { code: 'ks-IN', label: 'کأشُر (Kashmiri)' },
  { code: 'ne-NP', label: 'नेपाली (Nepali)' },
  { code: 'sd-PK', label: 'سنڌي (Sindhi)' },
  { code: 'kok-IN', label: 'कोंकणी (Konkani)' },
  { code: 'doi-IN', label: 'डोगरी (Dogri)' },
  { code: 'mni-IN', label: 'মণিপুরী (Manipuri)' },
  { code: 'brx-IN', label: 'बोडो (Bodo)' },
  { code: 'bho-IN', label: 'भोजपुरी (Bhojpuri)' },
  { code: 'hne-IN', label: 'छत्तीसगढ़ी (Chhattisgarhi)' },
  { code: 'mag-IN', label: 'मगही (Magahi)' },
  { code: 'mwr-IN', label: 'मारवाड़ी (Marwari)' },
  { code: 'awa-IN', label: 'अवधी (Awadhi)' },
  { code: 'lus-IN', label: 'Mizo' },
  { code: 'kha-IN', label: 'Khasi' },
  { code: 'grt-IN', label: 'Garo' },
  { code: 'trp-IN', label: 'Kokborok' },
  { code: 'tcy-IN', label: 'ತುಳು (Tulu)' },
  { code: 'anp-IN', label: 'अंगिका (Angika)' },
  { code: 'mtr-IN', label: 'मेवाड़ी (Mewari)' },
  { code: 'bfy-IN', label: 'बघेली (Bagheli)' },
  { code: 'bns-IN', label: 'बुंदेली (Bundeli)' },
];

export { enUS, bnIN };
