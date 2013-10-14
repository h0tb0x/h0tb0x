.. _api/self:

====
Self
====

``/api/s``
==========

.. http:get:: /api/s

   Returns the self resource.

.. _api/friends:

=======
Friends
=======

``/api/f``
==========

.. http:get:: /api/f

   List friends.

.. http:post:: /api/f

   Add friend.

``/api/f/{fid}``
================

.. http:get:: /api/f/{fid}

   Get friend details.

.. http:put:: /api/f/{fid}

   Update friend details.

.. http:delete:: /api/f/{fid}

   Remove a friend.

===========
Collections
===========

``/api/c``
==========

.. http:get:: /api/c

   List collections.

.. http:post:: /api/c

   Create new collection.

``/api/c/{cid}``
================

.. http:get:: /api/c/{cid}

   Get collection details.

``/api/c/{cid}/w``
==================

.. http:get:: /api/c/{cid}/w

   List collection writers.

.. http:post:: /api/c/{cid}/w

   Add collection writer.

``/api/c/{cid}/w/{fid}``
========================

.. http:get:: /api/c/{cid}/w/{fid}

   Get collection writer details.

.. http:delete:: /api/c/{cid}/w/{fid}

   Remove a collection writer.

