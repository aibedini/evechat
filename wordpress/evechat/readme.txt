=== EveChat ===
Contributors: evechat
Tags: live chat, support, chat widget, woocommerce, rtl, persian
Requires at least: 5.8
Tested up to: 6.6
Requires PHP: 7.4
Stable tag: 1.0.0
License: MIT
License URI: https://opensource.org/licenses/MIT

Real-time customer support chat for WordPress, with full WooCommerce customer context.

== Description ==

EveChat adds the EveChat live-support widget to your WordPress site.

* Persian (RTL) widget out of the box, English available.
* Passes the logged-in user, page URL and (with WooCommerce) customer id,
  lifetime value, order count and last order to the support team.
* Shortcode `[evechat]` for a "chat with us" button.
* No external services beyond your own EveChat server.

You need an EveChat server (self-hosted, single Go binary) and its project key.

== Installation ==

1. Upload the `evechat` folder to `/wp-content/plugins/`.
2. Activate the plugin through the *Plugins* screen.
3. Go to *Settings → EveChat* and paste your EveChat URL (e.g. `https://chat.example.com`)
   and the project key (`pk_...`) from your dashboard.
4. Optionally place `[evechat]` anywhere you want a chat button.

== Frequently Asked Questions ==

= Does it work without WooCommerce? =
Yes. The WooCommerce integration is loaded only when WooCommerce is active.

= Are the secrets stored in WordPress? =
Only the public project key and the server URL. Message encryption and API keys
stay on your EveChat server.

= Is the widget blocking page load? =
No, `widget.js` is enqueued in the footer with `async` semantics through
`wp_enqueue_script`, and it renders in a shadow root.

== Changelog ==

= 1.0.0 =
* Initial release: settings screen, widget embed, WooCommerce context, shortcode.
