<?php
/**
 * Admin settings screen for EveChat.
 *
 * @package EveChat
 */

defined( 'ABSPATH' ) || exit;

/**
 * Register the settings page and fields.
 */
function evechat_register_settings() {
	register_setting(
		'evechat',
		'evechat_settings',
		array(
			'type'              => 'array',
			'sanitize_callback' => 'evechat_sanitize_settings',
			'default'           => array(),
		)
	);

	add_settings_section( 'evechat_main', __( 'Widget configuration', 'evechat' ), '__return_false', 'evechat' );

	add_settings_field(
		'evechat_api_url',
		__( 'EveChat URL', 'evechat' ),
		'evechat_field_text',
		'evechat',
		'evechat_main',
		array( 'key' => 'api_url', 'placeholder' => 'https://chat.example.com' )
	);
	add_settings_field(
		'evechat_project_key',
		__( 'Project key', 'evechat' ),
		'evechat_field_text',
		'evechat',
		'evechat_main',
		array( 'key' => 'project_key', 'placeholder' => 'pk_...' )
	);
	add_settings_field(
		'evechat_position',
		__( 'Position', 'evechat' ),
		'evechat_field_select',
		'evechat',
		'evechat_main',
		array( 'key' => 'position', 'options' => array( 'bottom-left' => 'Bottom left', 'bottom-right' => 'Bottom right' ) )
	);
	add_settings_field(
		'evechat_locale',
		__( 'Locale', 'evechat' ),
		'evechat_field_select',
		'evechat',
		'evechat_main',
		array( 'key' => 'locale', 'options' => array( 'fa' => 'فارسی', 'en' => 'English' ) )
	);
	add_settings_field(
		'evechat_auto_open',
		__( 'Open automatically', 'evechat' ),
		'evechat_field_checkbox',
		'evechat',
		'evechat_main',
		array( 'key' => 'auto_open' )
	);
}
add_action( 'admin_init', 'evechat_register_settings' );

/**
 * Add the options page.
 */
function evechat_admin_menu() {
	add_options_page(
		__( 'EveChat', 'evechat' ),
		__( 'EveChat', 'evechat' ),
		'manage_options',
		'evechat',
		'evechat_render_settings'
	);
}
add_action( 'admin_menu', 'evechat_admin_menu' );

/**
 * Sanitize the submitted settings.
 *
 * @param array $input Raw input.
 * @return array
 */
function evechat_sanitize_settings( $input ) {
	$input = is_array( $input ) ? $input : array();
	return array(
		'api_url'     => esc_url_raw( isset( $input['api_url'] ) ? $input['api_url'] : '' ),
		'project_key' => sanitize_text_field( isset( $input['project_key'] ) ? $input['project_key'] : '' ),
		'position'    => in_array( isset( $input['position'] ) ? $input['position'] : '', array( 'bottom-left', 'bottom-right' ), true )
			? $input['position']
			: 'bottom-left',
		'locale'      => in_array( isset( $input['locale'] ) ? $input['locale'] : '', array( 'fa', 'en' ), true ) ? $input['locale'] : 'fa',
		'auto_open'   => empty( $input['auto_open'] ) ? 0 : 1,
	);
}

/**
 * Text field renderer.
 *
 * @param array $args Field args.
 */
function evechat_field_text( $args ) {
	$settings = get_option( 'evechat_settings', array() );
	$key      = $args['key'];
	$value    = isset( $settings[ $key ] ) ? $settings[ $key ] : '';
	printf(
		'<input type="text" class="regular-text" dir="ltr" name="evechat_settings[%1$s]" value="%2$s" placeholder="%3$s" />',
		esc_attr( $key ),
		esc_attr( $value ),
		esc_attr( isset( $args['placeholder'] ) ? $args['placeholder'] : '' )
	);
}

/**
 * Select renderer.
 *
 * @param array $args Field args.
 */
function evechat_field_select( $args ) {
	$settings = get_option( 'evechat_settings', array() );
	$key      = $args['key'];
	$value    = isset( $settings[ $key ] ) ? $settings[ $key ] : '';
	echo '<select name="evechat_settings[' . esc_attr( $key ) . ']">';
	foreach ( $args['options'] as $option => $label ) {
		printf(
			'<option value="%1$s" %2$s>%3$s</option>',
			esc_attr( $option ),
			selected( $value, $option, false ),
			esc_html( $label )
		);
	}
	echo '</select>';
}

/**
 * Checkbox renderer.
 *
 * @param array $args Field args.
 */
function evechat_field_checkbox( $args ) {
	$settings = get_option( 'evechat_settings', array() );
	$key      = $args['key'];
	$value    = ! empty( $settings[ $key ] );
	printf(
		'<input type="checkbox" name="evechat_settings[%1$s]" value="1" %2$s />',
		esc_attr( $key ),
		checked( $value, true, false )
	);
}

/**
 * Render the settings page.
 */
function evechat_render_settings() {
	if ( ! current_user_can( 'manage_options' ) ) {
		return;
	}
	?>
	<div class="wrap">
		<h1><?php esc_html_e( 'EveChat widget', 'evechat' ); ?></h1>
		<p><?php esc_html_e( 'Paste the project key from your EveChat dashboard (Settings → Widget).', 'evechat' ); ?></p>
		<form method="post" action="options.php">
			<?php
			settings_fields( 'evechat' );
			do_settings_sections( 'evechat' );
			submit_button();
			?>
		</form>
		<hr />
		<h2><?php esc_html_e( 'Shortcode', 'evechat' ); ?></h2>
		<p><?php esc_html_e( 'Add a "Chat with us" button anywhere:', 'evechat' ); ?> <code>[evechat]</code></p>
		<p><code>[evechat text="گفتگو با پشتیبانی" class="my-button"]</code></p>
	</div>
	<?php
}
