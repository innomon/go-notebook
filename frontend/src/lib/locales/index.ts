import { enUS } from './en-US';
import { bnIN } from './bn-IN';

export const resources = {
  'en-US': { translation: enUS },
  'en-IN': { translation: enUS },
  'en-GB': { translation: enUS },
  'hi-IN': { translation: enUS },
  'bn-IN': { translation: bnIN },
  'bn-BD': { translation: bnIN },
  'mr-IN': { translation: enUS },
  'te-IN': { translation: enUS },
  'ta-IN': { translation: enUS },
  'gu-IN': { translation: enUS },
  'ur-PK': { translation: enUS },
  'ur-IN': { translation: enUS },
  'kn-IN': { translation: enUS },
  'or-IN': { translation: enUS },
  'ml-IN': { translation: enUS },
  'pa-IN': { translation: enUS },
  'as-IN': { translation: enUS },
  'mai-IN': { translation: enUS },
  'sat-IN': { translation: enUS },
  'ks-IN': { translation: enUS },
  'ne-NP': { translation: enUS },
  'sd-PK': { translation: enUS },
  'kok-IN': { translation: enUS },
  'doi-IN': { translation: enUS },
  'mni-IN': { translation: enUS },
  'brx-IN': { translation: enUS },
  'bho-IN': { translation: enUS },
  'hne-IN': { translation: enUS },
  'mag-IN': { translation: enUS },
  'mwr-IN': { translation: enUS },
  'awa-IN': { translation: enUS },
  'lus-IN': { translation: enUS },
  'kha-IN': { translation: enUS },
  'grt-IN': { translation: enUS },
  'trp-IN': { translation: enUS },
  'tcy-IN': { translation: enUS },
  'anp-IN': { translation: enUS },
  'mtr-IN': { translation: enUS },
  'bfy-IN': { translation: enUS },
  'bns-IN': { translation: enUS },
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
