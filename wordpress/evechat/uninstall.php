<?php
/**
 * Uninstall cleanup for EveChat.
 *
 * @package EveChat
 */

defined( 'WP_UNINSTALL_PLUGIN' ) || exit;

delete_option( 'evechat_settings' );

if ( is_multisite() ) {
	$sites = get_sites( array( 'number' => 500 ) );
	foreach ( $sites as $site ) {
		switch_to_blog( $site->blog_id );
		delete_option( 'evechat_settings' );
		restore_current_blog();
	}
}
