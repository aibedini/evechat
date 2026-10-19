// Gregorian <-> Jalali conversion with no external dependency.
// The authoritative path uses the browser's ICU Persian calendar; a small
// arithmetic fallback covers environments where Intl lacks that calendar.
import { copy } from "./copy.fa";

const JALALI_MONTHS = copy.dates.months;

interface JalaliDate {
  jy: number;
  jm: number;
  jd: number;
}

let formatter: Intl.DateTimeFormat | null = null;
let formatterBroken = false;

function persianFormatter(): Intl.DateTimeFormat | null {
  if (formatter || formatterBroken) return formatter;
  try {
    formatter = new Intl.DateTimeFormat("en-US-u-ca-persian", {
      year: "numeric",
      month: "numeric",
      day: "numeric",
      timeZone: "UTC",
    });
    // Probe once: some ICU builds silently fall back to Gregorian.
    const probe = formatter.formatToParts(new Date(Date.UTC(2026, 9, 17)));
    const year = probe.find((p) => p.type === "year");
    if (!year || parseInt(year.value, 10) < 1300 || parseInt(year.value, 10) > 1500) {
      formatter = null;
      formatterBroken = true;
    }
  } catch {
    formatter = null;
    formatterBroken = true;
  }
  return formatter;
}

const div = (a: number, b: number) => Math.floor(a / b);
const mod = (a: number, b: number) => a - Math.floor(a / b) * b;

/** Arithmetic fallback (Borkowski's 2820-year cycle), midnight JDN. */
function jalaliFromJDN(jdn: number): JalaliDate {
  const j2d = (jy: number, jm: number, jd: number) => {
    const epbase = jy - (jy >= 0 ? 474 : 473);
    const epyear = 474 + mod(epbase, 2820);
    return (
      jd +
      (jm <= 7 ? (jm - 1) * 31 : (jm - 1) * 30 + 6) +
      Math.floor((epyear * 682 - 110) / 2816) +
      (epyear - 1) * 365 +
      Math.floor(epbase / 2820) * 1029983 +
      (1948320 - 1)
    );
  };
  const depoch = jdn - j2d(475, 1, 1);
  const cycle = div(depoch, 1029983);
  const cyear = mod(depoch, 1029983);
  let ycycle: number;
  if (cyear === 1029982) {
    ycycle = 2820;
  } else {
    const aux1 = div(cyear, 366);
    const aux2 = mod(cyear, 366);
    ycycle = div(2134 * aux1 + 2816 * aux2 + 2815, 1028522) + aux1 + 1;
  }
  let jy = ycycle + 2820 * cycle + 474;
  if (jy <= 0) jy--;
  const yday = jdn - j2d(jy, 1, 1) + 1;
  const jm = yday <= 186 ? Math.ceil(yday / 31) : Math.ceil((yday - 6) / 30);
  const jd = jdn - j2d(jy, jm, 1) + 1;
  return { jy, jm, jd };
}

function fallbackToJalali(date: Date): JalaliDate {
  const utcMidnight = Date.UTC(date.getFullYear(), date.getMonth(), date.getDate());
  const jdn = Math.floor(utcMidnight / 86400000) + 2440588 - 1;
  return jalaliFromJDN(jdn);
}

/** Converts a JS Date to a Jalali {jy, jm, jd} triple. */
export function toJalali(date: Date): JalaliDate {
  const fmt = persianFormatter();
  if (!fmt) return fallbackToJalali(date);
  try {
    const utc = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
    const parts = fmt.formatToParts(utc);
    let jy = 0;
    let jm = 0;
    let jd = 0;
    for (const p of parts) {
      if (p.type === "year") jy = parseInt(p.value, 10);
      else if (p.type === "month") jm = parseInt(p.value, 10);
      else if (p.type === "day") jd = parseInt(p.value, 10);
    }
    if (jy && jm >= 1 && jm <= 12 && jd >= 1 && jd <= 31) return { jy, jm, jd };
  } catch {
    /* fall through */
  }
  return fallbackToJalali(date);
}

/** Builds a JS Date from a Jalali date (searches around Nowruz). */
export function fromJalali(jy: number, jm: number, jd: number): Date {
  const guess = new Date(jy + 621, 2, 21);
  for (let i = -400; i <= 400; i++) {
    const d = new Date(guess.getTime());
    d.setDate(guess.getDate() + i);
    const j = toJalali(d);
    if (j.jy === jy && j.jm === jm && j.jd === jd) return d;
  }
  return guess;
}

function pad(n: number): string {
  return n < 10 ? "0" + n : String(n);
}

const FA_DIGITS = Array.from({ length: 10 }, (_, i) => String.fromCharCode(0x06f0 + i));

function faDigits(s: string): string {
  return s.replace(/\d/g, (x) => FA_DIGITS[parseInt(x, 10)]);
}

/** 1405/07/26 with Persian digits. */
export function formatJalali(date: Date | number): string {
  const d = typeof date === "number" ? new Date(date) : date;
  if (isNaN(d.getTime())) return "—";
  const j = toJalali(d);
  return faDigits(`${j.jy}/${pad(j.jm)}/${pad(j.jd)}`);
}

/** 14:05 with Persian digits. */
export function formatJalaliTime(date: Date | number): string {
  const d = typeof date === "number" ? new Date(date) : date;
  if (isNaN(d.getTime())) return "—";
  return faDigits(`${pad(d.getHours())}:${pad(d.getMinutes())}`);
}

/** Long Jalali date, e.g. 26 Mehr 1405. */
export function formatJalaliLong(date: Date | number): string {
  const d = typeof date === "number" ? new Date(date) : date;
  if (isNaN(d.getTime())) return "—";
  const j = toJalali(d);
  return faDigits(`${j.jd} ${JALALI_MONTHS[j.jm - 1]} ${j.jy}`);
}

export { JALALI_MONTHS };
