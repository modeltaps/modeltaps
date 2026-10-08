import React, { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { marked } from 'marked';
import { sanitizeHtml } from 'utils/sanitize';
import 'assets/css/content-viewer.css';

/**
 * ContentViewer component for displaying Markdown or HTML content
 * 
 * @param {Object} props - Component props
 * @param {string} props.content - The content to display (Markdown, HTML, or URL)
 * @param {boolean} props.loading - Whether the content is loading
 * @param {string} props.errorMessage - Error message to display if loading fails
 * @param {Object} props.containerStyle - Additional styles for the container
 * @param {Object} props.contentStyle - Additional styles for the content
 * @param {number} props.iframeHeight - Height for iframe (when content is a URL)
 * @returns {React.ReactElement} The rendered component
 */
const ContentViewer = ({ 
  content, 
  loading = false, 
  errorMessage = '', 
  containerStyle = {}, 
  contentStyle = {},
  iframeHeight = '100vh'
}) => {
  const [parsedContent, setParsedContent] = useState('');
  const [isUrl, setIsUrl] = useState(false);

  useEffect(() => {
    if (!content) {
      setParsedContent('');
      setIsUrl(false);
      return;
    }

    // Check if content is a URL
    if (content.startsWith('http://') || content.startsWith('https://')) {
      setIsUrl(true);
      setParsedContent(content);
      return;
    }

    // Check if content is already HTML
    if (content.trim().startsWith('<') && content.includes('</')) {
      setIsUrl(false);
      setParsedContent(sanitizeHtml(content));
      return;
    }

    // Parse as Markdown
    try {
      const parsed = marked.parse(content);
      setParsedContent(sanitizeHtml(parsed));
      setIsUrl(false);
    } catch (error) {
      console.error('Error parsing markdown:', error);
      setParsedContent(sanitizeHtml(content)); // Fallback to raw content
      setIsUrl(false);
    }
  }, [content]);

  if (loading) {
    return (
      <div className="flex min-h-[200px] items-center justify-center" style={containerStyle}>
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  if (errorMessage) {
    return (
      <div className="flex min-h-[200px] items-center justify-center" style={containerStyle}>
        <p className="text-sm text-destructive">{errorMessage}</p>
      </div>
    );
  }

  if (!content) {
    return null;
  }

  return (
    <div className="overflow-hidden bg-transparent" style={containerStyle}>
      {isUrl ? (
        <iframe
          title="content-frame"
          src={parsedContent}
          style={{ width: '100%', height: iframeHeight, border: 'none', ...contentStyle }}
        />
      ) : (
        <div
          className="content-viewer [&_img]:h-auto [&_img]:max-w-full"
          style={{ fontSize: 'inherit', lineHeight: 1.6, ...contentStyle }}
          dangerouslySetInnerHTML={{ __html: parsedContent }}
        />
      )}
    </div>
  );
};

ContentViewer.propTypes = {
  content: PropTypes.string,
  loading: PropTypes.bool,
  errorMessage: PropTypes.string,
  containerStyle: PropTypes.object,
  contentStyle: PropTypes.object,
  iframeHeight: PropTypes.string
};

export default ContentViewer;
