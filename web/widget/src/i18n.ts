export type Locale = "fa" | "en";

interface Strings {
  title: string;
  greeting: string;
  placeholder: string;
  send: string;
  typing: string;
  connecting: string;
  preparing: string;
  sessionFailed: string;
  reconnecting: string;
  degraded: string;
  offline: string;
  offlineNote: string;
  pending: string;
  sent: string;
  read: string;
  failed: string;
  retry: string;
  close: string;
  poweredBy: string;
  csatTitle: string;
  csatComment: string;
  csatSubmit: string;
  csatThanks: string;
  loginRequired: string;
}

const fa: Strings = {
  title: "پشتیبانی",
  greeting: "سلام 👋 چطور می‌توانیم کمک کنیم؟",
  placeholder: "پیام خود را بنویسید…",
  send: "ارسال",
  typing: "در حال نوشتن…",
  connecting: "در حال اتصال…",
  preparing: "در حال آماده‌سازی…",
  sessionFailed: "آماده‌سازی گفتگو ناموفق بود؛ دوباره تلاش کنید",
  reconnecting: "اتصال دوباره…",
  degraded: "اتصال ضعیف — پیام‌ها پس از برقراری ارسال می‌شوند",
  offline: "قطع شد — پیام‌ها ذخیره می‌شوند",
  offlineNote: "پیام‌ها پس از اتصال ارسال می‌شوند",
  pending: "در صف ارسال",
  sent: "ارسال شد",
  read: "خوانده شد",
  failed: "ارسال نشد — برای تلاش دوباره بزنید",
  retry: "تلاش دوباره",
  close: "بستن",
  poweredBy: "قدرت‌گرفته از EveChat",
  csatTitle: "تجربه خود را امتیاز دهید",
  csatComment: "توضیح اختیاری",
  csatSubmit: "ثبت",
  csatThanks: "ممنون از بازخورد شما 🙏",
  loginRequired: "برای شروع گفتگو نام خود را وارد کنید",
};

const en: Strings = {
  ...fa,
  title: "Support",
  greeting: "Hi there 👋 How can we help?",
  placeholder: "Write a message…",
  send: "Send",
  typing: "typing…",
  connecting: "Connecting…",
  preparing: "Preparing…",
  sessionFailed: "Could not prepare the chat; tap to retry",
  reconnecting: "Reconnecting…",
  degraded: "Weak connection — messages will be sent once connected",
  offline: "Disconnected — messages are saved",
  offlineNote: "Messages will be sent once connected",
  pending: "Sending",
  sent: "Sent",
  read: "Read",
  failed: "Failed — tap to retry",
  retry: "Retry",
  close: "Close",
  poweredBy: "Powered by EveChat",
  csatTitle: "Rate your experience",
  csatComment: "Optional comment",
  csatSubmit: "Submit",
  csatThanks: "Thanks for your feedback 🙏",
  loginRequired: "Enter your name to start the chat",
};

const maps: Record<Locale, Strings> = { fa, en };

export function t(locale: string): Strings {
  return maps[(locale as Locale) in maps ? (locale as Locale) : "fa"];
}

export type { Strings };
