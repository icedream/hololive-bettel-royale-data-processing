// module.exports = {
//     webpack: {
//         configure: (webpackConfig) => {
//             // Add the following lines to handle 'crypto' and 'fs' dependencies
//             webpackConfig.resolve.fallback = {
//                 fs: require.resolve("browserify-fs"), // or 'empty' if you prefer an empty module
//                 crypto: require.resolve('crypto-browserify'),
//                 stream: require.resolve('stream-browserify'),
//             };

//             // Add the 'module' configuration for handling .wasm files
//             webpackConfig.module.rules.push({
//                 test: /\.wasm$/,
//                 type: 'javascript/auto',
//             });

//             return webpackConfig;
//         },
//     },
// };

// const path = require('path');
const NodePolyfillPlugin = require('node-polyfill-webpack-plugin');

module.exports = function override(webpackConfig, env) {

    // Add the following lines to handle 'crypto' and 'fs' dependencies
    webpackConfig.resolve.fallback = {
        // fs: require.resolve("browserify-fs"), // or 'empty' if you prefer an empty module
        // // buffer: require.resolve("buffer"),
        // // util: require.resolve("util"),
        // crypto: require.resolve('crypto-browserify'),
        // stream: require.resolve('stream-browserify'),
        fs: false,
        global: false,
        __filename: false,
        __dirname: false,
        path: false,
        crypto: false,
        stream: false,
    };
    
    webpackConfig.experiments = webpackConfig.experiments || {};
    webpackConfig.experiments.asyncWebAssembly = true;

    // Add the 'module' configuration for handling .wasm files
    // webpackConfig.module.rules.push({
    //     test: /\.wasm$/,
    //     // type: 'javascript/auto',
    //     // use: [
    //     //     {loader:'file-loader'},
    //     // ],
    //     type: 'webassembly/async',
    // });
    // webpackConfig.resolve.extensions.push('.wasm');
    // webpackConfig.plugins.push(new NodePolyfillPlugin());

    return webpackConfig;

    // /**
    //  * Add WASM support
    //  */

    // // Make file-loader ignore WASM files
    // const wasmExtensionRegExp = /\.wasm$/;
    // config.resolve.extensions.push('.wasm');
    // config.module.rules.forEach(rule => {
    //     (rule.oneOf || []).forEach(oneOf => {
    //         if (oneOf.loader && oneOf.loader.indexOf('file-loader') >= 0) {
    //             oneOf.exclude.push(wasmExtensionRegExp);
    //         }
    //     });
    // });

    // // Add a dedicated loader for WASM
    // config.module.rules.push({
    //     test: wasmExtensionRegExp,
    //     include: path.resolve(__dirname, 'src'),
    //     use: [{ loader: require.resolve('wasm-loader'), options: {} }]
    // });

    // return config;
};
