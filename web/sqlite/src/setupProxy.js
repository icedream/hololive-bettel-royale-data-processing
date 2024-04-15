const express = require('express');

/**
 * 
 * @param {express.Application} app 
 */
module.exports = function setupProxy(app) {
    app.use(
        '/sql',
        express.static(`${__dirname}/../../../sql`, {
            dotfiles: 'ignore',
            etag: true,
            extensions: ['db'],
            index: false,
            redirect: false,
            maxAge: '1m',
        }),
    );
    app.use(
        '/db',
        express.static(`${__dirname}/../../../db`, {
            dotfiles: 'ignore',
            etag: true,
            extensions: ['db'],
            index: false,
            redirect: false,
            maxAge: '1m',
        }),
    );
};