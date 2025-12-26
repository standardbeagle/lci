<?php
/**
 * WordPress Plugin/Theme Hooks Test File
 */

// Basic action hooks
add_action('init', 'my_plugin_init');
add_action('wp_enqueue_scripts', 'enqueue_plugin_assets');
add_action('admin_menu', 'add_admin_pages');

// Action hooks with priority
add_action('wp_head', 'add_custom_meta', 5);
add_action('wp_footer', 'add_footer_scripts', 99);

// Action hooks with array callbacks (class methods)
add_action('plugins_loaded', array($this, 'initialize_plugin'));
add_action('admin_init', [$this, 'register_settings']);

// Filter hooks
add_filter('the_content', 'modify_post_content');
add_filter('the_title', 'filter_title');

// Filter hooks with priority
add_filter('excerpt_length', 'custom_excerpt_length', 20);
add_filter('post_class', 'add_custom_post_classes', 10, 3);

// Filter hooks with array callbacks
add_filter('wp_nav_menu_items', array($this, 'add_menu_items'), 10, 2);
add_filter('body_class', [$this, 'add_body_classes']);

// Shortcode registration
add_shortcode('my_button', 'render_button_shortcode');
add_shortcode('contact_form', array($this, 'render_contact_form'));

// REST API route registration
register_rest_route('myplugin/v1', '/items', array(
    'methods' => 'GET',
    'callback' => 'get_items_handler',
    'permission_callback' => '__return_true',
));

register_rest_route('myplugin/v1', '/items/(?P<id>\d+)', array(
    'methods' => 'GET',
    'callback' => 'get_single_item',
));

// Callback function definitions
function my_plugin_init() {
    // Plugin initialization code
}

function enqueue_plugin_assets() {
    wp_enqueue_style('plugin-style', plugin_dir_url(__FILE__) . 'css/style.css');
    wp_enqueue_script('plugin-script', plugin_dir_url(__FILE__) . 'js/script.js', array('jquery'), '1.0.0', true);
}

function modify_post_content($content) {
    return $content . '<p>Modified by plugin</p>';
}

function render_button_shortcode($atts) {
    return '<button class="my-button">Click Me</button>';
}

function get_items_handler($request) {
    return rest_ensure_response(array('items' => array()));
}

// Plugin class with hooks
class My_Plugin {
    public function __construct() {
        add_action('init', array($this, 'register_post_types'));
        add_filter('post_type_link', array($this, 'custom_permalink'), 10, 2);
    }

    public function register_post_types() {
        register_post_type('product', array(
            'public' => true,
            'label' => 'Products',
        ));
    }

    public function custom_permalink($post_link, $post) {
        return $post_link;
    }
}
