<?php
/**
 * Plugin Name: My Awesome Plugin
 * Plugin URI: https://example.com/my-plugin
 * Description: This plugin does awesome things for WordPress sites.
 * Version: 2.1.0
 * Author: John Doe
 * Author URI: https://johndoe.com
 * License: GPL-2.0+
 * Text Domain: my-awesome-plugin
 * Requires at least: 5.8
 * Requires PHP: 7.4
 */

// Prevent direct access
defined('ABSPATH') || exit;

/**
 * Template Name: Full Width Page
 * Template Post Type: page, post
 */

/**
 * Template Name: Sidebar Layout
 * Template Post Type: page
 */

// Gutenberg block registration with string name
register_block_type('myplugin/hero-block', array(
    'render_callback' => 'render_hero_block',
    'attributes' => array(
        'title' => array('type' => 'string'),
        'subtitle' => array('type' => 'string'),
    ),
));

// Gutenberg block registration with path
register_block_type(__DIR__ . '/blocks/gallery-block');

// Block type from metadata
register_block_type_from_metadata(__DIR__ . '/blocks/testimonial');

// WP_Block_Type instantiation
$custom_block = new WP_Block_Type('myplugin/custom-cta', array(
    'render_callback' => 'render_cta_block',
    'category' => 'widgets',
));

// More hooks for testing
add_action('init', 'my_plugin_init');
add_filter('the_content', 'modify_content');

// Plugin class
class My_Awesome_Plugin {
    public function __construct() {
        add_action('wp_enqueue_scripts', array($this, 'enqueue_assets'));
    }

    public function enqueue_assets() {
        wp_enqueue_style('my-plugin-style', plugin_dir_url(__FILE__) . 'css/style.css');
    }
}

function render_hero_block($attributes) {
    return '<div class="hero-block">' . esc_html($attributes['title']) . '</div>';
}

function render_cta_block($attributes) {
    return '<div class="cta-block">Call to Action</div>';
}
