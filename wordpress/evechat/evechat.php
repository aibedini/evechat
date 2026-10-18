<?php
/**
 * Plugin Name:       EveChat
 * Plugin URI:        https://github.com/evechat/evechat
 * Description:       Adds the EveChat live-support widget to your WordPress site, with WooCommerce customer context.
 * Version:           1.0.0
 * Requires at least: 5.8
 * Requires PHP:      7.4
 * Author:            EveChat
 * License:           MIT
 * Text Domain:       evechat
 */

defined( 'ABSPATH' ) || exit;

define( 'EVECHAT_VERSION', '1.0.0' );
define( 'EVECHAT_PLUGIN_DIR', plugin_dir_path( __FILE__ ) );
define( 'EVECHAT_PLUGIN_URL', plugin_dir_url( __FILE__ ) );

require_once EVECHAT_PLUGIN_DIR . 'includes/settings.php';
require_once EVECHAT_PLUGIN_DIR . 'includes/shortcode.php';

if ( class_exists( 'WooCommerce' ) ) {
	require_once EVECHAT_PLUGIN_DIR . 'includes/woocommerce.php';
}

/**
 * Default widget configuration.
 *
 * @return array
 */
function evechat_default_config() {
	return array(
		'api_url'     => '',
		'project_key' => '',
		'position'    => 'bottom-left',
		'locale'      => 'fa',
		'auto_open'   => false,
		'visitor'     => array(
			'name'     => '',
			'email'    => '',
			'mobile'   => '',
			'metadata' => array(),
		),
	);
}

/**
 * Current configuration, filterable by other plugins.
 *
 * @return array
 */
function evechat_config() {
	$settings = get_option( 'evechat_settings', array() );
	$config   = wp_parse_args(
		is_array( $settings ) ? $settings : array(),
		array(
			'api_url'     => '',
			'project_key' => '',
			'position'    => 'bottom-left',
			'locale'      => 'fa',
			'auto_open'   => 0,
		)
	);

	$user = wp_get_current_user();
	$visitor = array(
		'name'     => $user->exists() ? $user->display_name : '',
		'email'    => $user->exists() ? $user->user_email : '',
		'metadata' => array(
			'wp_user_id'  => $user->exists() ? (int) $user->ID : 0,
			'wp_page'     => evechat_current_url(),
			'wp_logged_in' => $user->exists() ? 1 : 0,
		),
	);

	$config['visitor']  = $visitor;
	$config['auto_open'] = (bool) $config['auto_open'];

	/**
	 * Filter the widget configuration right before it is printed.
	 *
	 * @param array $config Widget config.
	 */
	return apply_filters( 'evechat_config', $config );
}

/**
 * Absolute URL of the current request (without query args we do not need).
 *
 * @return string
 */
function evechat_current_url() {
	global $wp;
	if ( isset( $wp ) && is_object( $wp ) && method_exists( $wp, 'request' ) ) {
		$path = $wp->request;
		return home_url( user_trailingslashit( $path ) );
	}
	return home_url( add_query_arg( array(), isset( $_SERVER['REQUEST_URI'] ) ? sanitize_text_field( wp_unslash( $_SERVER['REQUEST_URI'] ) ) : '' ) );
}

/**
 * Enqueue the widget on the public site.
 */
function evechat_enqueue_widget() {
	$config = evechat_config();
	if ( empty( $config['project_key'] ) || empty( $config['api_url'] ) ) {
		return;
	}
	$api = untrailingslashit( $config['api_url'] );

	wp_enqueue_script( 'evechat-widget', $api . '/widget.js', array(), EVECHAT_VERSION, true );

	$inline = 'window.EveChat = ' . wp_json_encode(
		array(
			'projectKey' => $config['project_key'],
			'apiUrl'     => $api,
			'position'   => $config['position'],
			'locale'     => $config['locale'],
			'autoOpen'   => $config['auto_open'],
			'visitor'    => $config['visitor'],
		)
	) . ';';
	wp_add_inline_script( 'evechat-widget', $inline, 'before' );
}
add_action( 'wp_enqueue_scripts', 'evechat_enqueue_widget' );

/**
 * Settings link on the plugins screen.
 *
 * @param array $links Existing links.
 * @return array
 */
function evechat_plugin_action_links( $links ) {
	$url = admin_url( 'options-general.php?page=evechat' );
	array_unshift( $links, '<a href="' . esc_url( $url ) . '">' . esc_html__( 'Settings', 'evechat' ) . '</a>' );
	return $links;
}
add_filter( 'plugin_action_links_' . plugin_basename( __FILE__ ), 'evechat_plugin_action_links' );
