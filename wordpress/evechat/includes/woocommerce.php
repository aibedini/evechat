<?php
/**
 * WooCommerce context: passes order and customer value into the widget.
 *
 * @package EveChat
 */

defined( 'ABSPATH' ) || exit;

/**
 * Enrich the widget config with WooCommerce customer data.
 *
 * @param array $config Widget config.
 * @return array
 */
function evechat_woocommerce_config( $config ) {
	$user_id = get_current_user_id();
	if ( ! $user_id ) {
		return $config;
	}

	$customer = new WC_Customer( $user_id );
	$config['visitor']['name']  = $customer->get_display_name() ? $customer->get_display_name() : $config['visitor']['name'];
	$config['visitor']['email'] = $customer->get_email() ? $customer->get_email() : $config['visitor']['email'];

	$orders = wc_get_orders(
		array(
			'customer_id' => $user_id,
			'limit'       => 1,
			'orderby'     => 'date',
			'order'       => 'DESC',
		)
	);
	$last_order = ! empty( $orders ) ? $orders[0] : null;

	$config['visitor']['metadata']['wc_customer_id']   = $user_id;
	$config['visitor']['metadata']['wc_customer_ltv']  = (float) $customer->get_total_spent();
	$config['visitor']['metadata']['wc_orders_count']  = (int) $customer->get_order_count();

	if ( $last_order ) {
		$config['visitor']['metadata']['wc_last_order_id']    = $last_order->get_id();
		$config['visitor']['metadata']['wc_last_order_total'] = (float) $last_order->get_total();
		$config['visitor']['metadata']['wc_last_order_plan']  = evechat_order_plan( $last_order );
	}

	return $config;
}
add_filter( 'evechat_config', 'evechat_woocommerce_config' );

/**
 * Human readable summary of the ordered products (plan hint).
 *
 * @param WC_Order $order Order object.
 * @return string
 */
function evechat_order_plan( $order ) {
	$names = array();
	foreach ( $order->get_items() as $item ) {
		$names[] = $item->get_name();
	}
	$names = array_slice( array_filter( $names ), 0, 3 );
	return implode( ', ', $names );
}

/**
 * Add the order id to the visitor context right after checkout so a support
 * conversation can reference it.
 *
 * @param int $order_id Order id.
 */
function evechat_thankyou_context( $order_id ) {
	$order = wc_get_order( $order_id );
	if ( ! $order ) {
		return;
	}
	$context = array(
		'wc_order_id'    => (int) $order_id,
		'wc_order_total' => (float) $order->get_total(),
		'wc_order_plan'  => evechat_order_plan( $order ),
	);
	printf(
		'<script>window.ec=window.ec||function(){(ec.q=ec.q||[]).push(arguments)};ec("identify",%s);</script>',
		wp_json_encode(
			array(
				'metadata' => $context,
			)
		)
	);
}
add_action( 'woocommerce_thankyou', 'evechat_thankyou_context', 20 );
