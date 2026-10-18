<?php
/**
 * "Chat with us" shortcode.
 *
 * @package EveChat
 */

defined( 'ABSPATH' ) || exit;

/**
 * Render a button that opens the EveChat widget.
 *
 * @param array $atts Shortcode attributes.
 * @return string
 */
function evechat_shortcode( $atts ) {
	$atts = shortcode_atts(
		array(
			'text'  => __( 'گفتگو با پشتیبانی', 'evechat' ),
			'class' => 'evechat-button',
		),
		$atts,
		'evechat'
	);

	return sprintf(
		'<button type="button" class="%1$s" onclick="if(window.ec){ec(&quot;open&quot;)}else{window.ec=window.ec||function(){(ec.q=ec.q||[]).push(arguments)};ec(&quot;open&quot;)}">%2$s</button>',
		esc_attr( $atts['class'] ),
		esc_html( $atts['text'] )
	);
}
add_shortcode( 'evechat', 'evechat_shortcode' );
